#!/bin/sh
set -e

# ── Configuration from environment ──────────────────────────────────────────
DATA_DIR="${LATTICE_DATA_DIR:-/app/data}"
CONFIG_DIR="${LATTICE_CONFIG_DIR:-/etc/lattice}"
ADMIN_USER="${LATTICE_ADMIN_USER:-admin}"
ADMIN_PASS="${LATTICE_ADMIN_PASS:-changeme}"
JWT_SECRET="${LATTICE_JWT_SECRET:-$(tr -dc 'a-zA-Z0-9' < /dev/urandom | head -c 32)}"
K3S_KUBECONFIG="${K3S_KUBECONFIG:-/etc/rancher/k3s/k3s.yaml}"

# ── Log helper ──────────────────────────────────────────────────────────────
log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }

# ═════════════════════════════════════════════════════════════════════════════
# Step 1: Start k3s in background
# ═════════════════════════════════════════════════════════════════════════════
log "Starting k3s control plane..."

mkdir -p /var/log
k3s server \
    --disable traefik \
    --disable servicelb \
    > /var/log/k3s.log 2>&1 &
K3S_PID=$!

log "k3s PID: $K3S_PID"

# ═════════════════════════════════════════════════════════════════════════════
# Step 2: Wait for K8s API server
# ═════════════════════════════════════════════════════════════════════════════
export KUBECONFIG="$K3S_KUBECONFIG"

log "Waiting for K8s API server to become ready..."
for i in $(seq 1 120); do
    if kubectl get nodes > /dev/null 2>&1; then
        log "K8s API server is ready (${i}s)"
        break
    fi
    if [ $i -eq 120 ]; then
        log "ERROR: K8s API server failed to start within 120s"
        log "Check k3s log: cat /var/log/k3s.log"
        exit 1
    fi
    sleep 1
done

# ═════════════════════════════════════════════════════════════════════════════
# Step 3: Install CRDs
# ═════════════════════════════════════════════════════════════════════════════
log "Installing Lattice CRDs..."
kubectl apply -f /var/lib/rancher/k3s/server/manifests/crds/ 2>&1 || {
    log "WARNING: kubectl apply CRDs failed, k3s may still be processing them"
}

# ═════════════════════════════════════════════════════════════════════════════
# Step 4: Wait for CRDs to be established
# ═════════════════════════════════════════════════════════════════════════════
log "Waiting for Lattice CRDs to be established..."
for i in $(seq 1 60); do
    if kubectl get crd latticepeers.alattice.io > /dev/null 2>&1; then
        log "CRDs are ready (${i}s)"
        break
    fi
    if [ $i -eq 60 ]; then
        log "ERROR: CRDs not established within 60s"
        kubectl get crd 2>&1 || true
        exit 1
    fi
    sleep 1
done

# ═════════════════════════════════════════════════════════════════════════════
# Step 5: Bootstrap K8s resources
# ═════════════════════════════════════════════════════════════════════════════
log "Bootstrapping K8s resources..."

# Create namespace (idempotent)
kubectl create namespace lattice-system 2>/dev/null || true

# Create default LatticeGlobalIPPool (required for IPAM to work)
if ! kubectl get latticeglobalippool lattice-ip-pool -n lattice-system > /dev/null 2>&1; then
    kubectl apply -f - <<EOF
apiVersion: alattice.io/v1alpha1
kind: LatticeGlobalIPPool
metadata:
  name: lattice-ip-pool
  namespace: lattice-system
spec:
  cidr: 10.0.0.0/8
  subnetMask: 24
EOF
    log "Created default LatticeGlobalIPPool"
fi

# ═════════════════════════════════════════════════════════════════════════════
# Step 6: Generate latticed configuration
# ═════════════════════════════════════════════════════════════════════════════
mkdir -p "$CONFIG_DIR" "$DATA_DIR"

if [ ! -f "$CONFIG_DIR/lattice.yaml" ]; then
    log "Generating latticed configuration..."
    cat > "$CONFIG_DIR/lattice.yaml" <<EOF
app:
  listen: :8080
  name: "Lattice"
  env: "production"
  init_admins:
    - username: "$ADMIN_USER"
      password: "$ADMIN_PASS"
jwt:
  secret: "$JWT_SECRET"
  expire_hours: 24
signaling-url: "nats://localhost:4222"
database:
  dsn: "$DATA_DIR/lattice.db"
EOF
    log "Configuration written to $CONFIG_DIR/lattice.yaml"
else
    log "Using existing configuration at $CONFIG_DIR/lattice.yaml"
fi

# ═════════════════════════════════════════════════════════════════════════════
# Step 7: Start latticed (all-in-one: NATS + API + UI + K8s controller)
# ═════════════════════════════════════════════════════════════════════════════
log "Starting latticed (NATS + API + UI + K8s controller)..."
log "  Dashboard: http://localhost:8080"
log "  NATS:      nats://localhost:4222"
log "  Admin:     $ADMIN_USER / $ADMIN_PASS"

export LATTICE_CONFIG_DIR="$CONFIG_DIR"
export KUBECONFIG="$K3S_KUBECONFIG"

# Start latticed in background
/usr/bin/latticed &
LATTICED_PID=$!

# ── Wait for either process to exit (POSIX-compatible) ──────────────────────
while true; do
    kill -0 $K3S_PID 2>/dev/null || {
        log "k3s exited unexpectedly"
        kill $LATTICED_PID 2>/dev/null || true
        wait $K3S_PID
        exit $?
    }
    kill -0 $LATTICED_PID 2>/dev/null || {
        log "latticed exited"
        kill $K3S_PID 2>/dev/null || true
        wait $LATTICED_PID
        exit $?
    }
    sleep 2
done
