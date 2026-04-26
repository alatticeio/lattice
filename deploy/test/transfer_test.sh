#!/usr/bin/env bash
# transfer_test.sh
#
# 在带有 wf0 接口的容器之间随机传输数据，每隔 INTERVAL 秒运行一轮。
# 精确校验发送/接收字节数是否一致，所有记录写入 CSV。
#
# 用法:
#   ./transfer_test.sh                           # 测试所有发现的节点
#   ./transfer_test.sh --workspace wf-a1ac5a3a   # 只测某个 workspace 内的节点
#   ./transfer_test.sh --filter pod-a            # 按容器名关键字过滤
#   ./transfer_test.sh --once                    # 只运行一轮（调试）
#   ./transfer_test.sh --summary [--last N]      # 打印 CSV 统计（最近 N 轮）
#   ./transfer_test.sh --pairs N                 # 每轮随机选 N 对（默认 5）
#   ./transfer_test.sh --min-mb M --max-mb M     # 随机数据量范围 MB（默认 1-20）

set -euo pipefail

# ── 默认参数 ──────────────────────────────────────────────────────────────────
INTERVAL=30
PAIRS_PER_ROUND=5
MIN_MB=1
MAX_MB=20
WG_IFACE=wf0
BASE_PORT=20000
PORT_RANGE=9000
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CSV_FILE="${SCRIPT_DIR}/transfer_records.csv"
LOG_FILE="${SCRIPT_DIR}/transfer_test.log"
FILTER=""
WORKSPACE=""
RUN_ONCE=false
SUMMARY_MODE=false
SUMMARY_LAST=0

# ── 参数解析 ──────────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --workspace)   WORKSPACE="$2";       shift 2 ;;
    --filter)      FILTER="$2";          shift 2 ;;
    --pairs)       PAIRS_PER_ROUND="$2"; shift 2 ;;
    --min-mb)      MIN_MB="$2";          shift 2 ;;
    --max-mb)      MAX_MB="$2";          shift 2 ;;
    --interval)    INTERVAL="$2";        shift 2 ;;
    --once)        RUN_ONCE=true;        shift ;;
    --summary)     SUMMARY_MODE=true;    shift ;;
    --last)        SUMMARY_LAST="$2";    shift 2 ;;
    --csv)         CSV_FILE="$2";        shift 2 ;;
    *) echo "未知参数: $1"; exit 1 ;;
  esac
done

# ── 工具函数 ──────────────────────────────────────────────────────────────────

now_ts() { date '+%Y-%m-%d %H:%M:%S'; }

log() {
  local line="[$(now_ts)] $*"
  echo "$line"
  echo "$line" >> "$LOG_FILE"
}

# 毫秒时间戳（兼容 macOS BSD date）
ms_now() {
  python3 -c "import time; print(int(time.time()*1000))" 2>/dev/null \
    || echo "$(( $(date +%s) * 1000 ))"
}

# 毫秒 -> "X.XXXs"
ms_to_sec() {
  local ms=$1
  python3 -c "print(f'{$ms/1000:.3f}')" 2>/dev/null || echo "$ms"
}

# bytes + ms -> MB/s
throughput_mbps() {
  local bytes=$1 ms=$2
  python3 -c "
b, m = $bytes, $ms
print(f'{(b/1048576)/(m/1000):.2f}' if m > 0 else '0.00')
" 2>/dev/null || echo "0.00"
}

# ── 统计模式 ──────────────────────────────────────────────────────────────────

show_summary() {
  if [[ ! -f "$CSV_FILE" ]]; then
    echo "CSV 文件不存在: $CSV_FILE"; exit 0
  fi

  local min_round=0
  if [[ $SUMMARY_LAST -gt 0 ]]; then
    local max_round
    max_round=$(tail -n +2 "$CSV_FILE" | cut -d',' -f2 | sort -n | tail -1)
    min_round=$(( max_round - SUMMARY_LAST + 1 ))
  fi

  echo ""
  echo "═══════════════════════════════════════════════════════════════════════"
  printf "  %-5s  %-22s  %-22s  %8s  %10s  %10s  %7s  %s\n" \
    "轮次" "发送节点" "接收节点" "大小(MB)" "已发(B)" "已收(B)" "耗时(s)" "状态"
  echo "───────────────────────────────────────────────────────────────────────"

  local total_sent=0 total_recv=0 ok=0 fail=0

  while IFS=',' read -r ts round src dst smb sent recv dur tput status; do
    [[ "$round" == "round" ]] && continue
    [[ $min_round -gt 0 && "$round" -lt "$min_round" ]] && continue
    printf "  %-5s  %-22s  %-22s  %8s  %10s  %10s  %7s  %s\n" \
      "$round" "${src:0:22}" "${dst:0:22}" "$smb" "$sent" "$recv" "$dur" "$status"
    total_sent=$(( total_sent + sent ))
    total_recv=$(( total_recv + recv ))
    [[ "$status" == "OK" ]] && ok=$(( ok+1 )) || fail=$(( fail+1 ))
  done < <(tail -n +2 "$CSV_FILE" | sort -t',' -k2,2n)

  echo "───────────────────────────────────────────────────────────────────────"
  printf "  %-5s  %-22s  %-22s  %8s  %10s  %10s  %7s  OK:%-4d FAIL:%d\n" \
    "合计" "" "" "" "$total_sent" "$total_recv" "" "$ok" "$fail"
  echo "═══════════════════════════════════════════════════════════════════════"
  echo ""

  # 按节点对聚合（用 awk 避免关联数组）
  echo "  节点对汇总:"
  echo "  ─────────────────────────────────────────────────────────────────"
  printf "  %-22s  %-22s  %6s  %14s  %14s\n" "Src" "Dst" "次数" "累计发(B)" "累计收(B)"
  echo "  ─────────────────────────────────────────────────────────────────"

  tail -n +2 "$CSV_FILE" \
    | awk -F',' -v min="$min_round" '
      $2+0 < min && min > 0 { next }
      $2 == "round" { next }
      {
        key = $3 "|" $4
        cnt[key]++
        sent[key] += $6
        recv[key] += $7
      }
      END {
        for (k in cnt) {
          n = split(k, a, "|")
          printf "  %-22s  %-22s  %6d  %14d  %14d\n",
            substr(a[1],1,22), substr(a[2],1,22), cnt[k], sent[k], recv[k]
        }
      }
    ' | sort
  echo ""
}

if $SUMMARY_MODE; then show_summary; exit 0; fi

# ── 节点发现 ──────────────────────────────────────────────────────────────────

log "发现带有 $WG_IFACE 接口的容器..."

NODE_CONTAINERS=()
NODE_IPS=()
NODE_LABELS=()

while IFS= read -r cname; do
  [[ -n "$WORKSPACE" && "$cname" != *"$WORKSPACE"* ]] && continue
  [[ -n "$FILTER"    && "$cname" != *"$FILTER"*    ]] && continue

  ip=$(docker exec "$cname" ip -4 addr show "$WG_IFACE" 2>/dev/null \
       | grep -oE '([0-9]{1,3}\.){3}[0-9]{1,3}' | head -1 || true)
  [[ -z "$ip" ]] && continue

  label=$(echo "$cname" | grep -oE 'pod-[a-z0-9]+' | head -1 || echo "node${#NODE_CONTAINERS[@]}")
  label="${label}(${ip})"

  NODE_CONTAINERS+=("$cname")
  NODE_IPS+=("$ip")
  NODE_LABELS+=("$label")

  log "  #${#NODE_CONTAINERS[@]}  ${label}  ← $cname"
done < <(docker ps --format '{{.Names}}')

NODE_COUNT=${#NODE_CONTAINERS[@]}
if [[ $NODE_COUNT -lt 2 ]]; then
  log "错误: 只发现 $NODE_COUNT 个节点，至少需要 2 个"; exit 1
fi
log "共发现 $NODE_COUNT 个节点"

# ── 初始化 CSV ────────────────────────────────────────────────────────────────

if [[ ! -f "$CSV_FILE" ]]; then
  echo "timestamp,round,src_label,dst_label,size_mb,sent_bytes,recv_bytes,duration_s,throughput_mbps,status" \
    > "$CSV_FILE"
  log "创建 CSV: $CSV_FILE"
fi

# ── 单次传输 ──────────────────────────────────────────────────────────────────

do_transfer() {
  local round=$1 src_idx=$2 dst_idx=$3 size_mb=$4

  local src_c="${NODE_CONTAINERS[$src_idx]}"
  local dst_c="${NODE_CONTAINERS[$dst_idx]}"
  local dst_ip="${NODE_IPS[$dst_idx]}"
  local src_lbl="${NODE_LABELS[$src_idx]}"
  local dst_lbl="${NODE_LABELS[$dst_idx]}"

  local sent_bytes=$(( size_mb * 1048576 ))
  local port=$(( BASE_PORT + RANDOM % PORT_RANGE ))
  local rx_file="/tmp/wf_rx_${port}_$$"

  # 1. 启动接收端（后台），wc -c 精确计字节
  docker exec -d "$dst_c" sh -c \
    "nc -l -p ${port} | wc -c > ${rx_file}" 2>/dev/null || {
    log "  [ERROR] 无法在 $dst_lbl 启动 nc listener (port=$port)"
    return
  }

  sleep 0.3  # 等待 nc 就绪

  # 2. 发送端：通过 wf0 隧道发送随机数据，记录耗时
  local t0 t1
  t0=$(ms_now)
  docker exec "$src_c" sh -c \
    "dd if=/dev/urandom bs=1M count=${size_mb} 2>/dev/null | nc -w 15 ${dst_ip} ${port}" \
    2>/dev/null || true
  t1=$(ms_now)
  local elapsed_ms=$(( t1 - t0 ))

  sleep 0.5  # 等待接收端 wc 写入文件

  # 3. 读取接收端统计
  local recv_bytes
  recv_bytes=$(docker exec "$dst_c" sh -c \
    "cat ${rx_file} 2>/dev/null | tr -d '[:space:]'" 2>/dev/null || echo "0")
  recv_bytes="${recv_bytes:-0}"
  # 确保是纯数字
  [[ "$recv_bytes" =~ ^[0-9]+$ ]] || recv_bytes=0

  # 4. 清理临时文件
  docker exec "$dst_c" rm -f "$rx_file" 2>/dev/null || true

  # 5. 判断准确性
  local status
  if [[ "$recv_bytes" -eq "$sent_bytes" ]]; then
    status="OK"
  else
    local diff=$(( sent_bytes - recv_bytes ))
    status="MISMATCH(diff=${diff})"
  fi

  local dur_s tput
  dur_s=$(ms_to_sec "$elapsed_ms")
  tput=$(throughput_mbps "$recv_bytes" "$elapsed_ms")

  # 6. 写入 CSV
  echo "$(now_ts),${round},${src_lbl},${dst_lbl},${size_mb},${sent_bytes},${recv_bytes},${dur_s},${tput},${status}" \
    >> "$CSV_FILE"

  # 7. 终端日志
  local flag="✓"
  [[ "$status" != "OK" ]] && flag="✗"
  log "  $flag  $(printf '%-22s' "$src_lbl") → $(printf '%-22s' "$dst_lbl")  ${size_mb}MB  recv=${recv_bytes}B  ${dur_s}s  ${tput}MB/s  [${status}]"
}

# ── 随机不重复节点对（bash 3 兼容，不用关联数组）────────────────────────────

pick_pairs() {
  local count=$1
  local seen="" result="" n=0 attempts=0

  while [[ $n -lt $count && $attempts -lt 300 ]]; do
    local s=$(( RANDOM % NODE_COUNT ))
    local d=$(( RANDOM % NODE_COUNT ))
    local key=" ${s}_${d} "
    if [[ $s -ne $d && "$seen" != *"$key"* ]]; then
      seen="${seen}${key}"
      result="${result} ${s}:${d}"
      n=$(( n + 1 ))
    fi
    attempts=$(( attempts + 1 ))
  done
  echo "$result"
}

# ── 主循环 ────────────────────────────────────────────────────────────────────

round=0
log "开始传输测试  节点数=${NODE_COUNT}  每轮=${PAIRS_PER_ROUND}对  间隔=${INTERVAL}s  数据量=${MIN_MB}-${MAX_MB}MB"
log "CSV: $CSV_FILE"

while true; do
  round=$(( round + 1 ))
  log "━━━ 第 ${round} 轮  $(now_ts) ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  local_max=$(( NODE_COUNT * (NODE_COUNT - 1) ))
  pairs_wanted=$(( PAIRS_PER_ROUND < local_max ? PAIRS_PER_ROUND : local_max ))

  for pair in $(pick_pairs "$pairs_wanted"); do
    src="${pair%%:*}"
    dst="${pair##*:}"
    size=$(( MIN_MB + RANDOM % (MAX_MB - MIN_MB + 1) ))
    do_transfer "$round" "$src" "$dst" "$size"
  done

  $RUN_ONCE && break

  log "等待 ${INTERVAL}s..."
  sleep "$INTERVAL"
done
