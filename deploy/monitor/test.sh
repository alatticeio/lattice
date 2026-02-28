while true; do
#  # 模拟产生一个 30-60ms 之间的随机延迟
#  LATENCY=$(( ( RANDOM % 30 )  + 30 ))
  # 模拟一个不稳定的网络：大部分时间 40ms，偶尔跳到 200ms
  if [ $((RANDOM % 10)) -eq 0 ]; then
    LATENCY=$(( ( RANDOM % 100 ) + 150 )) # 模拟突发高延迟
  else
    LATENCY=$(( ( RANDOM % 10 ) + 35 ))   # 模拟正常状态
  fi
  curl -s -d "wireflow_test_latency{peer=\"macbook-pro.local\", type=\"icmp\"} $LATENCY" \
       http://localhost:8429/api/v1/import/prometheus
  echo "Data sent: $LATENCY ms"
  sleep 5
done