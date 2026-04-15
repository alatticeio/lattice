#!/bin/bash
# Wireflow One-Click Quick Start
# Usage: curl -sSL https://raw.githubusercontent.com/wireflowio/wireflow/master/hack/quickstart.sh | bash
set -euo pipefail

# ═══════════════════════════ Config ═══════════════════════════
CLUSTER_NAME="wireflow"
NAMESPACE="wireflow-system"
GITHUB_RAW="https://raw.githubusercontent.com/wireflowio/wireflow/master"
API_PORT=8080
NATS_PORT=4222
HEALTH_TIMEOUT=120   # seconds to wait for pod ready
# ═══════════════════════════════════════════════════════════════

# ── colours ──────────────────────────────────────────────────
info()  { echo -e "\033[32m[INFO]\033[0m  $*"; }
warn()  { echo -e "\033[33m[WARN]\033[0m  $*"; }
err()   { echo -e "\033[31m[ERROR]\033[0m $*" >&2; exit 1; }
ok()    { echo -e "\033[32m  ✓\033[0m  $*"; }
step()  { echo ""; echo -e "\033[1;34m━━  $*\033[0m"; }

echo ""
echo "┌──────────────────────────────────────────────────────────┐"
echo "│         🌊  Wireflow  One-Click Quick Start              │"
echo "└──────────────────────────────────────────────────────────┘"

# ─────────────────────────────────────────────────────────────
# STEP 1  Pre-flight check
# ─────────────────────────────────────────────────────────────
step "Step 1/4  Pre-flight check"

# -- Docker --
if ! command -v docker &>/dev/null; then
    err "Docker is not installed. Install it from https://docs.docker.com/get-docker/ and re-run."
fi
if ! docker info &>/dev/null 2>&1; then
    err "Docker daemon is not running. Start Docker and re-run."
fi
ok "Docker $(docker version --format '{{.Server.Version}}' 2>/dev/null || echo 'OK')"

# -- k3d --
if ! command -v k3d &>/dev/null; then
    info "k3d not found — installing..."
    curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
fi
ok "k3d $(k3d version 2>/dev/null | head -1 || echo 'OK')"

# -- kubectl --
if ! command -v kubectl &>/dev/null; then
    info "kubectl not found — installing..."
    K8S_STABLE=$(curl -Ls https://dl.k8s.io/release/stable.txt)
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
    curl -Lo /tmp/kubectl "https://dl.k8s.io/release/${K8S_STABLE}/bin/${OS}/${ARCH}/kubectl"
    chmod +x /tmp/kubectl && sudo mv /tmp/kubectl /usr/local/bin/
fi
ok "kubectl $(kubectl version --client --short 2>/dev/null | head -1 || echo 'OK')"

# -- port availability --
check_port() {
    local port=$1 label=$2
    if lsof -iTCP:"${port}" -sTCP:LISTEN -t &>/dev/null 2>&1; then
        err "Port ${port} (${label}) is already in use. Free it and re-run."
    fi
    ok "Port ${port} (${label}) is available"
}
# Only check if this is a fresh install (cluster does not yet exist)
if ! k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}[[:space:]]"; then
    check_port $API_PORT  "Dashboard / API"
    check_port $NATS_PORT "NATS signaling"
fi

# ─────────────────────────────────────────────────────────────
# STEP 2  Cluster creation
# ─────────────────────────────────────────────────────────────
step "Step 2/4  Setting up k3d cluster"

if k3d cluster list 2>/dev/null | grep -q "^${CLUSTER_NAME}[[:space:]]"; then
    warn "Cluster '${CLUSTER_NAME}' already exists — reusing it."
    k3d cluster start "${CLUSTER_NAME}" 2>/dev/null || true
else
    info "Creating k3d cluster '${CLUSTER_NAME}'..."
    k3d cluster create "${CLUSTER_NAME}" \
        --servers 1 --agents 0 \
        -p "${API_PORT}:${API_PORT}@loadbalancer" \
        -p "${NATS_PORT}:${NATS_PORT}@loadbalancer" \
        --k3s-arg "--disable=traefik@server:0"
fi

k3d kubeconfig merge "${CLUSTER_NAME}" --kubeconfig-merge-default >/dev/null

info "Waiting for cluster node to be Ready..."
kubectl wait --for=condition=Ready node --all --timeout=60s >/dev/null
ok "Cluster is ready"

# ─────────────────────────────────────────────────────────────
# STEP 3  Artifacts loading  (CRDs → RBAC → Service → Deploy)
# ─────────────────────────────────────────────────────────────
step "Step 3/4  Deploying Wireflow control plane"

CRD_BASE="${GITHUB_RAW}/config/crd/bases"
info "Applying CRDs..."
for crd in \
    wireflowcontroller.wireflow.run_wireflowinvitations.yaml \
    wireflowcontroller.wireflow.run_wireflownetworks.yaml \
    wireflowcontroller.wireflow.run_wireflowpolicies.yaml \
    wireflowcontroller.wireflow.run_wireflowpeers.yaml \
    wireflowcontroller.wireflow.run_wireflowglobalippools.yaml \
    wireflowcontroller.wireflow.run_wireflowendpoints.yaml \
    wireflowcontroller.wireflow.run_wireflowsubnetallocations.yaml \
    wireflowcontroller.wireflow.run_wireflowenrollmenttokens.yaml; do
    kubectl apply -f "${CRD_BASE}/${crd}" >/dev/null
done
ok "CRDs applied"

info "Creating namespace wireflow-system"
kubectl create ns wireflow-system > /dev/null
ok "Namespace wireflow-system created"

info "Applying app manifest (RBAC + Service + Deployment)..."
kubectl apply -f "${GITHUB_RAW}/deploy/quickstart/wireflow-all-in-one.yaml" >/dev/null
ok "Manifests applied"

# ─────────────────────────────────────────────────────────────
# STEP 4  Post-install  (health check + agent connection string)
# ─────────────────────────────────────────────────────────────
step "Step 4/4  Waiting for Wireflow to become ready"

info "Waiting for wireflowd pod (timeout ${HEALTH_TIMEOUT}s)..."
kubectl wait --for=condition=Ready pod \
    -l app=wireflowd \
    -n "${NAMESPACE}" \
    --timeout="${HEALTH_TIMEOUT}s" || \
    warn "Pod readiness timed out — it may still be pulling the image. Re-run or check: kubectl get pods -n ${NAMESPACE}"

# API health probe
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
    warn "API is not yet responding. Check pod logs: kubectl logs -l app=wireflowd -n ${NAMESPACE}"
fi

# Generate initial token
INITIAL_TOKEN=""
if command -v wireflow &>/dev/null; then
    info "Generating initial agent token via wireflow CLI..."
    TOKEN_OUTPUT=$(wireflow \
        --signaling-url "nats://localhost:${NATS_PORT}" \
        token create quickstart \
        -n default \
        --limit 100 \
        --expiry 720h 2>&1) || true
    # extract the token value (long alphanumeric string)
    INITIAL_TOKEN=$(echo "${TOKEN_OUTPUT}" | grep -oE '[A-Za-z0-9_\-]{24,}' | tail -1 || true)
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

if [ -n "${INITIAL_TOKEN}" ]; then
echo "║   One-click Agent connect:                              ║"
echo "║                                                          ║"
printf "║     wireflow up \\\\                                        ║\n"
printf "║       --signaling-url nats://localhost:%-18s║\n" "${NATS_PORT} \\"
printf "║       --token %-43s║\n" "${INITIAL_TOKEN}"
echo "╚══════════════════════════════════════════════════════════╝"
else
echo "║   To connect an agent, first create a token:            ║"
echo "║                                                          ║"
printf "║     wireflow token create my-token \\\\                    ║\n"
printf "║       --signaling-url nats://localhost:%-18s║\n" "${NATS_PORT} \\"
printf "║       -n default --limit 10 --expiry 168h               ║\n"
echo "║                                                          ║"
echo "║     wireflow up --token <TOKEN>  \\                       ║"
printf "║       --signaling-url nats://localhost:%-18s║\n" "${NATS_PORT}"
echo "╚══════════════════════════════════════════════════════════╝"
fi

echo ""
echo "  Useful commands:"
echo "    kubectl get pods -n ${NAMESPACE}"
echo "    kubectl get wfpeer -A"
echo ""
echo "  To uninstall:"
echo "    k3d cluster delete ${CLUSTER_NAME}"
echo ""
