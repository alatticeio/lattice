# Local Test-README Script Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract `test-readme-script.yml` into `scripts/test-readme.sh` so the full README CLI test suite can be run locally with `bash scripts/test-readme.sh`.

**Architecture:** A single self-contained shell script holds all test logic. It resolves images (local build or GHCR pull), runs all README steps end-to-end, and on failure preserves containers for debugging. The CI workflow is simplified to just call this script, passing image tags via env vars.

**Tech Stack:** bash, Docker, existing `make docker-build` Makefile targets.

---

### Task 1: Create `scripts/test-readme.sh` — skeleton, image resolution, cleanup trap

**Files:**
- Create: `scripts/test-readme.sh`

- [ ] **Step 1: Create the script with argument parsing and constants**

```bash
#!/usr/bin/env bash
# test-readme.sh — Run README CLI test suite locally or in CI.
#
# Usage:
#   bash scripts/test-readme.sh              # build k3s image if not present
#   bash scripts/test-readme.sh --force-build # always rebuild both images
#
# Env vars (set by CI to override image resolution):
#   K3S_IMAGE   — lattice-k3s image to use (e.g. ghcr.io/alatticeio/lattice-k3s:pr-42)
#   AGENT_IMAGE — lattice agent image to use (e.g. ghcr.io/alatticeio/lattice:pr-42)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── Constants ────────────────────────────────────────────────────────────────
SERVER_URL="http://localhost:8080"
KUBECTL="docker exec lattice-k3s kubectl"
LOCAL_K3S_IMAGE="lattice-k3s:local"
LOCAL_AGENT_IMAGE="ghcr.io/alatticeio/lattice:local"
GHCR_AGENT_IMAGE="ghcr.io/alatticeio/lattice:latest"

# ── Argument parsing ─────────────────────────────────────────────────────────
FORCE_BUILD=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --force-build) FORCE_BUILD=true; shift ;;
    *) echo "Unknown flag: $1"; exit 1 ;;
  esac
done
```

- [ ] **Step 2: Add image resolution logic**

Append to `scripts/test-readme.sh`:

```bash
# ── Image resolution ─────────────────────────────────────────────────────────
# Priority: env var > local image > build/pull
resolve_k3s_image() {
  if [[ -n "${K3S_IMAGE:-}" ]]; then
    echo "K3S: using env K3S_IMAGE=$K3S_IMAGE"
    return
  fi
  if [[ "$FORCE_BUILD" == true ]] || ! docker image inspect "$LOCAL_K3S_IMAGE" > /dev/null 2>&1; then
    echo "K3S: building $LOCAL_K3S_IMAGE from source..."
    docker build -f "$REPO_ROOT/deploy/k3s/Dockerfile" -t "$LOCAL_K3S_IMAGE" "$REPO_ROOT"
  else
    echo "K3S: reusing existing $LOCAL_K3S_IMAGE (use --force-build to rebuild)"
  fi
  K3S_IMAGE="$LOCAL_K3S_IMAGE"
}

resolve_agent_image() {
  if [[ -n "${AGENT_IMAGE:-}" ]]; then
    echo "Agent: using env AGENT_IMAGE=$AGENT_IMAGE"
    return
  fi
  if [[ "$FORCE_BUILD" == true ]] || ! docker image inspect "$LOCAL_AGENT_IMAGE" > /dev/null 2>&1; then
    echo "Agent: building $LOCAL_AGENT_IMAGE from source..."
    cd "$REPO_ROOT" && make docker-build SERVICE=lattice TAG=local
  else
    echo "Agent: reusing existing $LOCAL_AGENT_IMAGE (use --force-build to rebuild)"
  fi
  AGENT_IMAGE="$LOCAL_AGENT_IMAGE"
}

resolve_k3s_image
resolve_agent_image
```

- [ ] **Step 3: Add cleanup trap**

Append to `scripts/test-readme.sh`:

```bash
# ── Cleanup trap ─────────────────────────────────────────────────────────────
TEST_PASSED=false

cleanup() {
  if [[ "$TEST_PASSED" == true ]]; then
    echo "=== Cleaning up ==="
    docker rm -f lattice-k3s wf-agent-a wf-agent-b 2>/dev/null || true
    docker network rm wf-test-net 2>/dev/null || true
    echo "Done."
  else
    echo ""
    echo "=== TEST FAILED — containers preserved for debugging ==="
    echo "  docker logs lattice-k3s"
    echo "  docker logs wf-agent-a"
    echo "  docker logs wf-agent-b"
    echo "To clean up manually:"
    echo "  docker rm -f lattice-k3s wf-agent-a wf-agent-b"
    echo "  docker network rm wf-test-net"
  fi
}
trap cleanup EXIT
```

- [ ] **Step 4: Make the script executable and verify it parses args**

```bash
chmod +x scripts/test-readme.sh
bash scripts/test-readme.sh --help 2>&1 || true   # should print "Unknown flag: --help"
bash -n scripts/test-readme.sh                     # syntax check — no output = OK
```

- [ ] **Step 5: Commit**

```bash
git add scripts/test-readme.sh
git commit -m "feat(scripts): add test-readme.sh skeleton with image resolution and cleanup trap"
```

---

### Task 2: Add startup phase — container launch and readiness waiting

**Files:**
- Modify: `scripts/test-readme.sh`

- [ ] **Step 1: Add container startup and K8s API wait**

Append to `scripts/test-readme.sh` (after the trap):

```bash
# ── Phase 1: Start lattice-k3s ───────────────────────────────────────────────
echo "=== Starting lattice-k3s container ==="
docker run -d \
  --name lattice-k3s \
  --privileged \
  -p 8080:8080 \
  "$K3S_IMAGE"
echo "lattice-k3s started"

echo "Waiting for K8s API..."
for i in $(seq 1 120); do
  if $KUBECTL get nodes > /dev/null 2>&1; then
    echo "K8s API ready after ${i}s"
    break
  fi
  if [ "$i" -eq 120 ]; then
    echo "ERROR: K8s API not ready within 120s"
    docker logs lattice-k3s 2>&1 | tail -30
    exit 1
  fi
  sleep 1
done

echo "Waiting for CRDs..."
for i in $(seq 1 60); do
  if $KUBECTL get crd latticepeers.alattice.io > /dev/null 2>&1; then
    echo "CRDs ready after ${i}s"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: CRDs not ready within 60s"
    exit 1
  fi
  sleep 1
done
```

- [ ] **Step 2: Add latticed API readiness wait**

Append to `scripts/test-readme.sh`:

```bash
echo "Waiting for Lattice login API (${SERVER_URL}/api/v1/users/login)..."
for i in $(seq 1 90); do
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 3 \
    -X POST "${SERVER_URL}/api/v1/users/login" \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"123456","client":"cli"}' 2>/dev/null) || true
  if [ "$HTTP_CODE" != "000" ] && [ "$HTTP_CODE" != "502" ] && [ "$HTTP_CODE" != "503" ]; then
    echo "Lattice login API ready (HTTP $HTTP_CODE) after ${i}s"
    break
  fi
  if [ "$i" -eq 90 ]; then
    echo "ERROR: Lattice login API not ready within 90s"
    docker logs lattice-k3s 2>&1 | tail -30
    exit 1
  fi
  sleep 2
done
```

- [ ] **Step 3: Add login and CLI setup**

Append to `scripts/test-readme.sh`:

```bash
# ── Login ────────────────────────────────────────────────────────────────────
echo ">>> POST /api/v1/users/login"
LOGIN_RESP=$(curl -s --max-time 10 -X POST "${SERVER_URL}/api/v1/users/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"123456","client":"cli"}')
AUTH_TOKEN=$(echo "$LOGIN_RESP" | python3 -c \
  "import sys,json; print(json.load(sys.stdin)['data']['token'])" 2>/dev/null || true)
[[ -n "$AUTH_TOKEN" ]] || { echo "ERROR: failed to obtain auth token (response: $LOGIN_RESP)"; exit 1; }
echo "Auth token obtained (${#AUTH_TOKEN} chars)"

LATTICE="docker run --rm --network host -e LATTICE_AUTH_TOKEN=${AUTH_TOKEN} ${AGENT_IMAGE}"
echo "lattice CLI: $LATTICE"
```

- [ ] **Step 4: Syntax check**

```bash
bash -n scripts/test-readme.sh   # no output = OK
```

- [ ] **Step 5: Commit**

```bash
git add scripts/test-readme.sh
git commit -m "feat(scripts): add startup, readiness wait, and login phases to test-readme.sh"
```

---

### Task 3: Add Steps 1–2 (workspace and token)

**Files:**
- Modify: `scripts/test-readme.sh`

- [ ] **Step 1: Add workspace create/list**

Append to `scripts/test-readme.sh`:

```bash
# ════════════════════════════════════════════════════════════════════════════
# Step 1: Create a workspace
# ════════════════════════════════════════════════════════════════════════════
echo "=== Step 1: workspace ==="
echo ">>> lattice workspace add dev --display-name Development --server-url $SERVER_URL"
WS_OUTPUT=$($LATTICE workspace add dev \
  --display-name "Development" \
  --server-url "$SERVER_URL")
echo "$WS_OUTPUT"

NAMESPACE=$(echo "$WS_OUTPUT" | awk '/namespace:/ {print $2}')
[[ -n "$NAMESPACE" ]] || { echo "Failed to extract namespace"; exit 1; }
echo "Workspace created — namespace=$NAMESPACE"

echo ">>> lattice workspace list --server-url $SERVER_URL"
LIST=$($LATTICE workspace list --server-url "$SERVER_URL")
echo "$LIST"
echo "$LIST" | grep -q "$NAMESPACE" \
  && echo "Namespace '$NAMESPACE' visible in list" \
  || { echo "Workspace not found in list"; exit 1; }
```

- [ ] **Step 2: Add token create/list**

Append to `scripts/test-readme.sh`:

```bash
# ════════════════════════════════════════════════════════════════════════════
# Step 2: Create an enrollment token
# ════════════════════════════════════════════════════════════════════════════
echo "=== Step 2: enrollment token ==="
echo ">>> lattice token create dev-team -n $NAMESPACE --limit 10 --expiry 168h --server-url $SERVER_URL"
$LATTICE token create dev-team \
  -n "$NAMESPACE" \
  --limit 10 \
  --expiry 168h \
  --server-url "$SERVER_URL"
echo "Token created"

echo ">>> lattice token list -n $NAMESPACE --server-url $SERVER_URL"
TOKEN_LIST=$($LATTICE token list -n "$NAMESPACE" --server-url "$SERVER_URL")
echo "$TOKEN_LIST"

TOKEN=$(echo "$TOKEN_LIST" | awk '/^TOKEN/{found=1; next} found && NF>0 {print $1; exit}')
[[ -n "$TOKEN" ]] || { echo "Failed to extract token"; exit 1; }
echo "Token extracted: ${TOKEN:0:8}..."
```

- [ ] **Step 3: Syntax check**

```bash
bash -n scripts/test-readme.sh
```

- [ ] **Step 4: Commit**

```bash
git add scripts/test-readme.sh
git commit -m "feat(scripts): add Steps 1-2 (workspace and token) to test-readme.sh"
```

---

### Task 4: Add Steps 3–4 (agents and policy)

**Files:**
- Modify: `scripts/test-readme.sh`

- [ ] **Step 1: Add namespace wait and agent startup**

Append to `scripts/test-readme.sh`:

```bash
# ════════════════════════════════════════════════════════════════════════════
# Step 3: Start agents
# ════════════════════════════════════════════════════════════════════════════
echo "Waiting for operator to create workspace namespace $NAMESPACE..."
for i in $(seq 1 30); do
  $KUBECTL get ns "$NAMESPACE" &>/dev/null && break || true
  echo "  Waiting for namespace $NAMESPACE... ($i/30)"
  sleep 2
done
$KUBECTL get ns "$NAMESPACE" \
  && echo "Namespace $NAMESPACE ready" \
  || { echo "Namespace not created"; $KUBECTL get ns; exit 1; }

echo "=== Step 3: start agents ==="
docker network create wf-test-net

AGENT_API="http://host.docker.internal:8080"

echo ">>> docker run -d --name wf-agent-a ..."
docker run -d \
  --name wf-agent-a \
  --restart unless-stopped \
  --privileged \
  --network wf-test-net \
  --add-host=host.docker.internal:host-gateway \
  -e LATTICE_APP_ID=ci-wf-agent-a \
  "$AGENT_IMAGE" \
  up \
  --server-url "$AGENT_API" \
  --token "$TOKEN" \
  --level debug

echo ">>> docker run -d --name wf-agent-b ..."
docker run -d \
  --name wf-agent-b \
  --restart unless-stopped \
  --privileged \
  --network wf-test-net \
  --add-host=host.docker.internal:host-gateway \
  -e LATTICE_APP_ID=ci-wf-agent-b \
  "$AGENT_IMAGE" \
  up \
  --server-url "$AGENT_API" \
  --token "$TOKEN" \
  --level debug

echo "Both agent containers started on wf-test-net"
```

- [ ] **Step 2: Add agent registration wait and LatticePeer IP allocation wait**

Append to `scripts/test-readme.sh`:

```bash
echo "Waiting for both agents to register..."
for i in $(seq 1 36); do
  NODE_COUNT=$($LATTICE workspace list --server-url "$SERVER_URL" 2>/dev/null \
    | awk '/dev/ {print $4}' || echo 0)
  echo "  Nodes: ${NODE_COUNT:-0}  (attempt $i/36)"
  [[ "${NODE_COUNT:-0}" -ge 2 ]] && break || true
  sleep 5
done

FINAL=$($LATTICE workspace list --server-url "$SERVER_URL")
echo "$FINAL"
NODE_COUNT=$(echo "$FINAL" | awk '/dev/ {print $4}')
[[ "${NODE_COUNT:-0}" -ge 2 ]] \
  && echo "Both agents registered (nodes=$NODE_COUNT)" \
  || { echo "Expected >=2 nodes, got ${NODE_COUNT:-0}"; docker logs wf-agent-a; exit 1; }

echo "Waiting for both LatticePeers to have AllocatedAddress (max 90s)..."
for i in $(seq 1 30); do
  COUNT=$($KUBECTL get wfpeer -n "$NAMESPACE" \
    -o jsonpath='{.items[*].status.allocatedAddress}' 2>/dev/null \
    | tr ' ' '\n' | grep -v '^$' | wc -l || echo 0)
  echo "  Peers with IP: ${COUNT} (attempt $i/30)"
  [[ "${COUNT}" -ge 2 ]] && break || true
  sleep 3
done
COUNT=$($KUBECTL get wfpeer -n "$NAMESPACE" \
  -o jsonpath='{.items[*].status.allocatedAddress}' 2>/dev/null \
  | tr ' ' '\n' | grep -v '^$' | wc -l || echo 0)
[[ "${COUNT}" -ge 2 ]] \
  && echo "Both peers have WireGuard IPs" \
  || { echo "LatticePeer IP allocation timed out"; $KUBECTL get wfpeer -n "$NAMESPACE" -o yaml || true; exit 1; }
```

- [ ] **Step 3: Add policy steps**

Append to `scripts/test-readme.sh`:

```bash
# ════════════════════════════════════════════════════════════════════════════
# Step 4: Allow traffic between peers
# ════════════════════════════════════════════════════════════════════════════
echo "=== Step 4: policies ==="
echo ">>> lattice policy allow-all -n $NAMESPACE --server-url $SERVER_URL"
$LATTICE policy allow-all -n "$NAMESPACE" --server-url "$SERVER_URL"
echo "allow-all policy applied"

echo ">>> lattice policy add my-policy -n $NAMESPACE --action ALLOW --server-url $SERVER_URL"
$LATTICE policy add my-policy \
  -n "$NAMESPACE" \
  --action ALLOW \
  --desc "allow all peer traffic" \
  --server-url "$SERVER_URL"
echo "my-policy added"

echo ">>> lattice policy list -n $NAMESPACE --server-url $SERVER_URL"
POLICIES=$($LATTICE policy list -n "$NAMESPACE" --server-url "$SERVER_URL")
echo "$POLICIES"
echo "$POLICIES" | grep -q "allow-all" \
  && echo "allow-all present" || { echo "allow-all missing"; exit 1; }
echo "$POLICIES" | grep -q "my-policy" \
  && echo "my-policy present" || { echo "my-policy missing"; exit 1; }
```

- [ ] **Step 4: Syntax check**

```bash
bash -n scripts/test-readme.sh
```

- [ ] **Step 5: Commit**

```bash
git add scripts/test-readme.sh
git commit -m "feat(scripts): add Steps 3-4 (agents and policy) to test-readme.sh"
```

---

### Task 5: Add Steps 5–6 (verify connectivity and cleanup resources)

**Files:**
- Modify: `scripts/test-readme.sh`

- [ ] **Step 1: Add Step 5 — status and ping verification**

Append to `scripts/test-readme.sh`:

```bash
# ════════════════════════════════════════════════════════════════════════════
# Step 5: Verify connectivity
# ════════════════════════════════════════════════════════════════════════════
echo "=== Step 5: verify connectivity ==="
echo "Waiting 20s for WireGuard peer config to be pushed and handshake to complete..."
sleep 20

echo "===== LatticePeer CRD status ====="
$KUBECTL get wfpeer -n "$NAMESPACE" -o wide 2>&1 || true

for agent in wf-agent-a wf-agent-b; do
  echo "--- $agent state ---"
  docker inspect "$agent" --format '{{.State.Status}} exitCode={{.State.ExitCode}} restarting={{.State.Restarting}}'
done

echo ">>> docker exec wf-agent-a /app/lattice status"
STATUS_A=$(docker exec wf-agent-a /app/lattice status 2>&1)
echo "$STATUS_A"
echo "$STATUS_A" | grep -q "Interface" \
  && echo "agent-a: Interface present" \
  || { echo "No interface info in agent-a status"; exit 1; }

echo ">>> docker exec wf-agent-b /app/lattice status"
STATUS_B=$(docker exec wf-agent-b /app/lattice status 2>&1)
echo "$STATUS_B"
echo "$STATUS_B" | grep -q "Interface" \
  && echo "agent-b: Interface present" \
  || { echo "No interface info in agent-b status"; exit 1; }

WG_IP_B=$(echo "$STATUS_B" | awk '/^Address/{print $3}' | head -1 | cut -d',' -f1 | cut -d'/' -f1)
if [[ -z "$WG_IP_B" ]]; then
  WG_IP_B=$(docker exec wf-agent-b \
    ip -4 addr show wf0 2>/dev/null \
    | awk '/inet /{print $2}' | cut -d'/' -f1 || true)
fi
echo "Agent B WireGuard IP: ${WG_IP_B:-(unknown)}"

if [[ -n "$WG_IP_B" ]]; then
  SUCCESS=0
  for i in $(seq 1 3); do
    echo "  ping attempt $i/3..."
    if docker exec wf-agent-a ping -c 3 -W 2 "$WG_IP_B" 2>&1; then
      SUCCESS=1; break
    fi
    sleep 5
  done
  [[ "$SUCCESS" -eq 1 ]] \
    && echo "Ping succeeded — WireGuard tunnel is working" \
    || { echo "Ping failed after retries"; docker logs wf-agent-a --tail=50; docker logs wf-agent-b --tail=50; exit 1; }
else
  echo "Could not determine Agent B WireGuard IP"
  docker logs wf-agent-b --tail=30
  exit 1
fi
```

- [ ] **Step 2: Add Step 6 — resource cleanup and success marker**

Append to `scripts/test-readme.sh`:

```bash
# ════════════════════════════════════════════════════════════════════════════
# Step 6: Clean up resources
# ════════════════════════════════════════════════════════════════════════════
echo "=== Step 6: remove resources ==="
echo ">>> lattice token remove $TOKEN -n $NAMESPACE --server-url $SERVER_URL"
$LATTICE token remove "$TOKEN" -n "$NAMESPACE" --server-url "$SERVER_URL"
REMAINING=$($LATTICE token list -n "$NAMESPACE" --server-url "$SERVER_URL")
echo "$REMAINING" | grep -q "^$TOKEN" \
  && { echo "Token still present"; exit 1; } \
  || echo "Token removed"

echo ">>> lattice policy remove my-policy -n $NAMESPACE --server-url $SERVER_URL"
$LATTICE policy remove my-policy -n "$NAMESPACE" --server-url "$SERVER_URL"
POLICIES=$($LATTICE policy list -n "$NAMESPACE" --server-url "$SERVER_URL")
echo "$POLICIES" | grep -q "my-policy" \
  && { echo "my-policy still present"; exit 1; } \
  || echo "my-policy removed"

echo ">>> lattice workspace remove $NAMESPACE --server-url $SERVER_URL"
$LATTICE workspace remove "$NAMESPACE" --server-url "$SERVER_URL"
WORKSPACES=$($LATTICE workspace list --server-url "$SERVER_URL")
echo "$WORKSPACES" | grep -q "$NAMESPACE" \
  && { echo "Workspace still present"; exit 1; } \
  || echo "Workspace removed"

echo ""
echo "=== ALL STEPS PASSED ==="
TEST_PASSED=true
```

- [ ] **Step 3: Final syntax check**

```bash
bash -n scripts/test-readme.sh   # no output = OK
wc -l scripts/test-readme.sh     # informational
```

- [ ] **Step 4: Commit**

```bash
git add scripts/test-readme.sh
git commit -m "feat(scripts): add Steps 5-6 (connectivity verify + resource cleanup) to test-readme.sh"
```

---

### Task 6: Simplify CI workflow to call the script

**Files:**
- Modify: `.github/workflows/test-readme-script.yml`

- [ ] **Step 1: Replace all the inline steps with a single script call**

Replace the content of `.github/workflows/test-readme-script.yml` with:

```yaml
name: Test README Scripts

# Validates every CLI command documented in README.md by running scripts/test-readme.sh.
# The same script can be run locally: bash scripts/test-readme.sh

on:
  pull_request:
    branches: [master, main]
  workflow_dispatch:

env:
  K3S_IMAGE: ghcr.io/alatticeio/lattice-k3s:${{ github.event_name == 'pull_request' && format('pr-{0}', github.event.pull_request.number) || 'latest' }}
  AGENT_IMAGE: ghcr.io/alatticeio/lattice:${{ github.event_name == 'pull_request' && format('pr-{0}', github.event.pull_request.number) || 'latest' }}

permissions:
  contents: read

jobs:
  test-readme-scripts:
    runs-on: ubuntu-latest
    timeout-minutes: 40

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Run README test script
        run: bash scripts/test-readme.sh

      - name: Collect failure logs
        if: failure()
        run: |
          echo "===== lattice-system pods ====="
          docker exec lattice-k3s kubectl get all -n lattice-system -o wide 2>/dev/null || true

          echo "===== latticed logs ====="
          docker exec lattice-k3s kubectl logs -n lattice-system \
            -l app.kubernetes.io/name=latticed --tail=300 2>/dev/null || true

          echo "===== Agent A docker logs ====="
          docker logs wf-agent-a --tail=100 2>/dev/null || true

          echo "===== Agent B docker logs ====="
          docker logs wf-agent-b --tail=100 2>/dev/null || true

          echo "===== lattice-k3s container logs ====="
          docker logs lattice-k3s --tail=100 2>/dev/null || true
```

- [ ] **Step 2: Verify the workflow is valid YAML**

```bash
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/test-readme-script.yml'))" \
  && echo "YAML valid" || echo "YAML invalid"
```

Expected output: `YAML valid`

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/test-readme-script.yml
git commit -m "fix(ci): simplify test-readme-script workflow to call scripts/test-readme.sh"
```

---

### Task 7: Local smoke test

**Files:** none (verification only)

- [ ] **Step 1: Verify image resolution without building**

```bash
# Should print help/error but NOT build anything
docker image inspect lattice-k3s:local > /dev/null 2>&1 \
  && echo "lattice-k3s:local exists — will be reused" \
  || echo "lattice-k3s:local not found — will build on first run"
```

- [ ] **Step 2: Run the script (first time — will build lattice-k3s:local)**

```bash
cd /path/to/lattice
bash scripts/test-readme.sh
```

Expected final output:
```
=== ALL STEPS PASSED ===
=== Cleaning up ===
Done.
```

- [ ] **Step 3: Run the script again (should reuse lattice-k3s:local)**

```bash
bash scripts/test-readme.sh
```

Verify that the log shows `reusing existing lattice-k3s:local` instead of building.

- [ ] **Step 4: Test --force-build flag**

```bash
bash scripts/test-readme.sh --force-build
```

Verify that the log shows `building lattice-k3s:local from source...` even though the image already exists.

- [ ] **Step 5: Final commit with push**

```bash
git push origin feat/mcp-tools-without-llm
```
