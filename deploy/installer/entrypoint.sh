#!/bin/bash
# Wireflow Bootstrapper — runs inside wireflow/installer container
# Usage:
#   docker run --rm -it \
#     -v /var/run/docker.sock:/var/run/docker.sock \
#     -v ~/.kube:/root/.kube \
#     wireflow/installer:latest
#
# Env vars:
#   CLEANUP=true          — destroy the cluster and exit
#   CLUSTER_NAME          — override cluster name (default: wireflow)
#   API_PORT              — override API port   (default: 8080)
#   NATS_PORT             — override NATS port  (default: 4222)
#   GITHUB_RAW            — override manifest base URL
#
# NOTE: This installer deploys all-in-one mode only (wireflowd with embedded NATS + SQLite).
# For separated/production deployments: kubectl apply -k config/wireflow/overlays/production
set -euo pipefail

# ═══════════════════════════ Config ═══════════════════════════
CLUSTER_NAME="${CLUSTER_NAME:-wireflow}"
NAMESPACE="wireflow-system"
API_PORT="${API_PORT:-8080}"
NATS_PORT="${NATS_PORT:-4222}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-120}"
GITHUB_RAW="${GITHUB_RAW:-https://raw.githubusercontent.com/wireflowio/wireflow/master}"
CLEANUP="${CLEANUP:-false}"
# ══════════════════════════════════════════════════════════════

# ── colours ───────────────────────────────────────────────────
info()  { echo -e "\033[32m[INFO]\033[0m  $*"; }
warn()  { echo -e "\033[33m[WARN]\033[0m  $*"; }
err()   { echo -e "\033[31m[ERROR]\033[0m $*" >&2; exit 1; }
ok()    { echo -e "\033[32m  ✓\033[0m  $*"; }
step()  { echo ""; echo -e "\033[1;34m━━  $*\033[0m"; }

echo ""
echo "┌──────────────────────────────────────────────────────────┐"
echo "│         🌊  Wireflow  Bootstrapper (Docker mode)         │"
echo "└──────────────────────────────────────────────────────────┘"

# ─────────────────────────────────────────────────────────────
# STEP 0  Environment detection — verify Docker socket
# ─────────────────────────────────────────────────────────────
step "Step 0  Environment detection"

# 1. 物理检查
if [ ! -S /var/run/docker.sock ]; then
    err "Socket file not found! Check your '-v' mount path."
fi

# 2. 读写检查
if [ ! -w /var/run/docker.sock ]; then
    echo "💡 [HINT] Permission denied on socket."
    echo "   Please run: 'sudo chmod 666 /var/run/docker.sock' on host"
    echo "   Or add '--privileged' to your docker run command."
    exit 1
fi

# 3. 通讯检查 (打印详细错误)
if ! docker info >/dev/null 2>&1; then
    echo "--- Raw Error from Docker CLI ---"
    docker info || true
    err "Docker daemon is unresponsive. Possible API mismatch or SELinux block."
fi
ok "Docker daemon is reachable"

if ! docker info >/dev/null 2>&1; then
    err "Cannot reach the Docker daemon.
  Make sure you mounted the socket:
    -v /var/run/docker.sock:/var/run/docker.sock"
fi
ok "Docker daemon is reachable ($(docker version --format '{{.Server.Version}}' 2>/dev/null || echo 'OK'))"

# ─────────────────────────────────────────────────────────────
# CLEANUP path
# ─────────────────────────────────────────────────────────────
if [ "${CLEANUP}" = "true" ]; then
    step "CLEANUP mode — deleting cluster '${CLUSTER_NAME}'"
    if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}[[:space:]]"; then
        k3d cluster delete "${CLUSTER_NAME}"
        ok "Cluster '${CLUSTER_NAME}' deleted."
    else
        warn "Cluster '${CLUSTER_NAME}' not found — nothing to delete."
    fi
    exit 0
fi

# ─────────────────────────────────────────────────────────────
# STEP 1  Port check (only for fresh installs)
# ─────────────────────────────────────────────────────────────
check_port() {
    local port=$1 label=$2
    # lsof is not available inside the container; check via /proc/net/tcp*
    if grep -qE "$(printf '%04X' "${port}")" /proc/net/tcp /proc/net/tcp6 2>/dev/null; then
        warn "Port ${port} (${label}) appears to be in use on the host — continuing anyway."
    else
        ok "Port ${port} (${label}) looks available"
    fi
}

# ─────────────────────────────────────────────────────────────
# STEP 2  Cluster creation / idempotency
# ─────────────────────────────────────────────────────────────
step "Step 1  Setting up k3d cluster"

CLUSTER_EXISTS=false
if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}[[:space:]]"; then
    CLUSTER_EXISTS=true
fi

if $CLUSTER_EXISTS; then
    # Prompt user: reset or incremental update
    ACTION="update"
    if [ -t 0 ]; then
        echo ""
        echo "  Cluster '${CLUSTER_NAME}' already exists."
        echo "  [r] Reset   — delete and recreate from scratch"
        echo "  [u] Update  — apply latest manifests to the existing cluster (default)"
        echo ""
        read -r -p "  Choice [r/U]: " CHOICE
        case "${CHOICE,,}" in
            r|reset) ACTION="reset" ;;
            *)        ACTION="update" ;;
        esac
    else
        warn "Non-interactive mode: existing cluster detected → incremental update."
    fi

    if [ "${ACTION}" = "reset" ]; then
        info "Deleting cluster '${CLUSTER_NAME}'..."
        k3d cluster delete "${CLUSTER_NAME}"
        CLUSTER_EXISTS=false
    else
        info "Reusing existing cluster '${CLUSTER_NAME}'..."
        k3d cluster start "${CLUSTER_NAME}" 2>/dev/null || true
    fi
fi

if ! $CLUSTER_EXISTS; then
    check_port "${API_PORT}"  "Dashboard / API"
    check_port "${NATS_PORT}" "NATS signaling"
    info "Creating k3d cluster '${CLUSTER_NAME}'..."
    k3d cluster create "${CLUSTER_NAME}" \
        --servers 1 --agents 0 \
        -p "${API_PORT}:${API_PORT}@loadbalancer" \
        -p "${NATS_PORT}:${NATS_PORT}@loadbalancer" \
        --k3s-arg "--disable=traefik@server:0"
fi

k3d kubeconfig merge "${CLUSTER_NAME}" --kubeconfig-merge-default >/dev/null
ok "Kubeconfig merged (using 0.0.0.0, host network)"

info "Waiting for cluster node to be Ready..."
kubectl wait --for=condition=Ready node --all --timeout=120s >/dev/null
ok "Cluster is ready"

# ─────────────────────────────────────────────────────────────
# STEP 3  Apply CRDs + all-in-one deployment
# Manifests are generated by `make build-installer` and committed
# to the repo at deploy/quickstart/. Fetched via GITHUB_RAW.
# ─────────────────────────────────────────────────────────────
step "Step 2  Deploying Wireflow control plane (all-in-one)"

info "Applying CRDs..."
kubectl apply -f "${GITHUB_RAW}/deploy/quickstart/wireflow-crds.yaml" >/dev/null
ok "CRDs applied"

info "Applying RBAC + Service + Deployment..."
kubectl apply -f "${GITHUB_RAW}/deploy/quickstart/wireflow-all-in-one.yaml" >/dev/null
ok "Manifests applied"

# ─────────────────────────────────────────────────────────────
# STEP 4  Health check
# ─────────────────────────────────────────────────────────────
step "Step 3  Waiting for Wireflow to become ready"

info "Waiting for wireflowd pod (timeout ${HEALTH_TIMEOUT}s)..."
kubectl wait --for=condition=Ready pod \
    -l app.kubernetes.io/name=wireflowd \
    -n "${NAMESPACE}" \
    --timeout="${HEALTH_TIMEOUT}s" || \
    warn "Pod readiness timed out — still pulling image? Check: kubectl get pods -n ${NAMESPACE}"

info "Probing API on localhost:${API_PORT} ..."
HEALTH_OK=false
for i in $(seq 1 30); do
    if curl -sf --max-time 2 "http://localhost:${API_PORT}/metrics" >/dev/null 2>&1; then
        HEALTH_OK=true
        break
    fi
    sleep 2
done

if $HEALTH_OK; then
    ok "API is reachable at http://localhost:${API_PORT}"
else
    warn "API not yet responding. Check: kubectl logs -l app=wireflowd -n ${NAMESPACE}"
fi

# ─────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║   🚀  Wireflow Control Plane is UP!                     ║"
echo "╠══════════════════════════════════════════════════════════╣"
printf "║   %-55s ║\n" "Dashboard  →  http://localhost:${API_PORT}"
printf "║   %-55s ║\n" "NATS       →  nats://localhost:${NATS_PORT}"
echo "╠══════════════════════════════════════════════════════════╣"
echo "║   To connect an agent:                                  ║"
printf "║     wireflow token create my-token \\\\                    ║\n"
printf "║       --signaling-url nats://localhost:%-18s║\n" "${NATS_PORT} \\"
printf "║       -n default --limit 10 --expiry 168h               ║\n"
echo "║                                                          ║"
printf "║     wireflow up --token <TOKEN> \\\\                       ║\n"
printf "║       --signaling-url nats://localhost:%-18s║\n" "${NATS_PORT}"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "  Useful commands:"
echo "    kubectl get pods -n ${NAMESPACE}"
echo "    kubectl get wfpeer -A"
echo ""
echo "  To uninstall:"
printf "    docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \\\n"
printf "      -e CLEANUP=true wireflow/installer:latest\n"
echo ""
