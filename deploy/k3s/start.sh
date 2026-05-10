#!/bin/sh
set -e

# ── Configuration from environment ──────────────────────────────────────────
DATA_DIR="${LATTICE_DATA_DIR:-/app/data}"
CONFIG_DIR="${LATTICE_CONFIG_DIR:-/etc/lattice}"
ADMIN_USER="${LATTICE_ADMIN_USER:-admin}"
ADMIN_PASS="${LATTICE_ADMIN_PASS:-123456}"
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
    --kubelet-arg=cgroups-per-qos=false \
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
kubectl apply -f /var/lib/lattice/crds/ 2>&1 || {
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

# Wait for the custom resource API endpoint to actually respond.
# CRDs can be "established" while the extension API server still isn't ready
# to serve requests, causing "unexpected EOF" on first kubectl apply.
log "Waiting for LatticeGlobalIPPool API endpoint to be ready..."
for i in $(seq 1 30); do
    if kubectl get latticeglobalippools > /dev/null 2>&1; then
        log "LatticeGlobalIPPool API ready (${i}s)"
        break
    fi
    if [ "$i" -eq 30 ]; then
        log "WARNING: LatticeGlobalIPPool API did not become ready within 30s, continuing anyway"
    fi
    log "  [${i}/30] LatticeGlobalIPPool API not ready yet, waiting..."
    sleep 1
done

# Create default LatticeGlobalIPPool with retry (guards against transient EOF)
if ! kubectl get latticeglobalippool lattice-ip-pool > /dev/null 2>&1; then
    CREATED=0
    for i in $(seq 1 5); do
        log "  Attempt $i/5: kubectl apply LatticeGlobalIPPool..."
        APPLY_OUT=$(kubectl apply -f - 2>&1 <<EOF
apiVersion: alattice.io/v1alpha1
kind: LatticeGlobalIPPool
metadata:
  name: lattice-ip-pool
spec:
  cidr: 10.0.0.0/8
  subnetMask: 24
EOF
        ) && {
            log "Created default LatticeGlobalIPPool: $APPLY_OUT"
            CREATED=1
            break
        } || {
            log "  Attempt $i/5 failed: $APPLY_OUT"
            log "  Retrying in 3s..."
            sleep 3
        }
    done
    if [ "$CREATED" -eq 0 ]; then
        log "WARNING: Failed to create LatticeGlobalIPPool after 5 retries, continuing anyway"
    fi
else
    log "LatticeGlobalIPPool lattice-ip-pool already exists, skipping"
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

# Start latticed in background, redirect output to log file
mkdir -p /var/log
/usr/bin/latticed > /var/log/latticed.log 2>&1 &
LATTICED_PID=$!
log "latticed PID: $LATTICED_PID"

# Wait for latticed HTTP API to be ready
log "Waiting for latticed HTTP API on :8080..."
for i in $(seq 1 60); do
    if wget -q -O /dev/null http://localhost:8080/ 2>/dev/null; then
        log "latticed HTTP API ready (${i}s)"
        break
    fi
    if [ "$i" -eq 60 ]; then
        log "WARNING: latticed HTTP API not ready within 60s"
        log "--- latticed log (last 30 lines) ---"
        tail -30 /var/log/latticed.log 2>/dev/null || true
    fi
    if [ "$((i % 10))" -eq 0 ]; then
        log "  [${i}/60] still waiting for :8080... latticed log tail:"
        tail -5 /var/log/latticed.log 2>/dev/null || true
    fi
    sleep 1
done

# ── Wait for either process to exit (POSIX-compatible) ──────────────────────
while true; do
    kill -0 $K3S_PID 2>/dev/null || {
        log "k3s exited unexpectedly"
        log "--- latticed log (last 50 lines) ---"
        tail -50 /var/log/latticed.log 2>/dev/null || true
        kill $LATTICED_PID 2>/dev/null || true
        wait $K3S_PID
        exit $?
    }
    kill -0 $LATTICED_PID 2>/dev/null || {
        log "latticed exited unexpectedly"
        log "--- latticed log (last 50 lines) ---"
        tail -50 /var/log/latticed.log 2>/dev/null || true
        kill $K3S_PID 2>/dev/null || true
        wait $LATTICED_PID
        exit $?
    }
    sleep 2
done
