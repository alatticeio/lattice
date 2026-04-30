#!/bin/bash
# Lattice One-Click Quick Start
# Usage: curl -sSL https://raw.githubusercontent.com/alatticeio/lattice/master/hack/quickstart.sh | bash
set -euo pipefail

# ═══════════════════════════ Config ═══════════════════════════
CLUSTER_NAME="lattice"
NAMESPACE="lattice-system"
GITHUB_RAW="https://raw.githubusercontent.com/alatticeio/lattice/master"
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
echo "│         🌊  Lattice  One-Click Quick Start              │"
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

# -- port availability (needed for port-forward later) --
check_port() {
    local port=$1 label=$2
    if lsof -iTCP:"${port}" -sTCP:LISTEN -t &>/dev/null 2>&1; then
        err "Port ${port} (${label}) is already in use. Free it and re-run."
    fi
    ok "Port ${port} (${label}) is available"
}
# Only check on fresh install
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
        --k3s-arg "--disable=traefik@server:0" \
        --port "4222:4222@loadbalancer"
fi

k3d kubeconfig merge "${CLUSTER_NAME}" --kubeconfig-merge-default >/dev/null

info "Waiting for cluster node to be Ready..."
kubectl wait --for=condition=Ready node --all --timeout=60s >/dev/null
ok "Cluster is ready"

# ─────────────────────────────────────────────────────────────
# STEP 3  Artifacts loading  (CRDs → RBAC → Service → Deploy)
# ─────────────────────────────────────────────────────────────
step "Step 3/4  Deploying Lattice control plane"

CRD_BASE="${GITHUB_RAW}/config/crd/bases"
info "Applying CRDs..."
for crd in \
    alattice.io_latticeinvitations.yaml \
    alattice.io_latticenetworks.yaml \
    alattice.io_latticenetworkpeerings.yaml \
    alattice.io_latticepolicies.yaml \
    alattice.io_latticepeers.yaml \
    alattice.io_latticeglobalippools.yaml \
    alattice.io_latticeendpoints.yaml \
    alattice.io_latticesubnetallocations.yaml \
    alattice.io_latticeenrollmenttokens.yaml \
    alattice.io_latticerelayservers.yaml \
    alattice.io_latticeclusters.yaml \
    alattice.io_latticeclusterpeerings.yaml; do
    kubectl apply -f "${CRD_BASE}/${crd}" >/dev/null
done
ok "CRDs applied"

info "Creating namespace lattice-system"
kubectl create ns lattice-system > /dev/null
ok "Namespace lattice-system created"

info "Applying app manifest (RBAC + Service + Deployment)..."
kubectl apply -f "${GITHUB_RAW}/deploy/quickstart/lattice-all-in-one.yaml" >/dev/null
ok "Manifests applied"

# ─────────────────────────────────────────────────────────────
# STEP 4  Post-install  (port-forward → health check → summary)
# ─────────────────────────────────────────────────────────────
step "Step 4/4  Waiting for Lattice to become ready"

info "Waiting for latticed pod (timeout ${HEALTH_TIMEOUT}s)..."
kubectl wait --for=condition=Ready pod \
    -l app=latticed \
    -n "${NAMESPACE}" \
    --timeout="${HEALTH_TIMEOUT}s" || \
    warn "Pod readiness timed out — it may still be pulling the image. Re-run or check: kubectl get pods -n ${NAMESPACE}"

# Generate initial token
INITIAL_TOKEN=""
if command -v lattice &>/dev/null; then
    info "Generating initial agent token via lattice CLI..."
    TOKEN_OUTPUT=$(lattice \
        --signaling-url "nats://localhost:${NATS_PORT}" \
        token create quickstart \
        -n default \
        --limit 100 \
        --expiry 720h 2>&1) || true
    INITIAL_TOKEN=$(echo "${TOKEN_OUTPUT}" | grep -oE '[A-Za-z0-9_\-]{24,}' | tail -1 || true)
fi

# ─────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║   🚀  Lattice Control Plane is UP!                     ║"
echo "╠══════════════════════════════════════════════════════════╣"
printf "║   %-55s ║\n" "Dashboard  →  http://localhost:${API_PORT}"
printf "║   %-55s ║\n" "NATS       →  nats://localhost:${NATS_PORT}"
echo "╠══════════════════════════════════════════════════════════╣"

if [ -n "${INITIAL_TOKEN}" ]; then
echo "║   One-click Agent connect:                              ║"
echo "║                                                          ║"
printf "║     lattice up \\\\                                        ║\n"
printf "║       --signaling-url nats://localhost:%-18s║\n" "${NATS_PORT} \\"
printf "║       --token %-43s║\n" "${INITIAL_TOKEN}"
echo "╚══════════════════════════════════════════════════════════╝"
else
echo "║   To connect an agent, first create a token:            ║"
echo "║                                                          ║"
printf "║     lattice token create my-token \\\\                    ║\n"
printf "║       --signaling-url nats://localhost:%-18s║\n" "${NATS_PORT} \\"
printf "║       -n default --limit 10 --expiry 168h               ║\n"
echo "║                                                          ║"
echo "║     lattice up --token <TOKEN>  \\                       ║"
printf "║       --signaling-url nats://localhost:%-18s║\n" "${NATS_PORT}"
echo "╚══════════════════════════════════════════════════════════╝"
fi

echo ""
echo "  Useful commands:"
echo "    kubectl get pods -n ${NAMESPACE}"
echo "    kubectl get wfpeer -A"
echo ""
echo "  To access services locally (ClusterIP), run:"
echo "    kubectl port-forward -n ${NAMESPACE} svc/lattice-api-service ${API_PORT}:${API_PORT} &"
echo "    kubectl port-forward -n ${NAMESPACE} svc/lattice-nats-service ${NATS_PORT}:${NATS_PORT} &"
echo ""
echo "  To uninstall:"
echo "    k3d cluster delete ${CLUSTER_NAME}"
echo ""
