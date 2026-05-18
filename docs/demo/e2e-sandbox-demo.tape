# Lattice Agent Sandbox — E2E Demo
# Generated from test/e2e/agent_sandbox_test.go (7 scenarios)
# Render: vhs < docs/demo/e2e-sandbox-demo.tape

Output docs/public/demo/sandbox-e2e.gif

Set Shell "zsh"
Set FontSize 14
Set FontFamily "JetBrains Mono"
Set Width 90
Set Height 28
Set Padding 60
Set Framerate 24

Hide

# ── Header ──────────────────────────────────────────────────────────────

Type@0.05 "# Lattice Agent Sandbox — E2E Demo"
Enter
Sleep 500ms
Type@0.05 "# 对照 test/e2e/agent_sandbox_test.go 7 个场景"
Enter
Enter
Sleep 1s

# ── Step 0: Check version ───────────────────────────────────────────────

Type@0.05 "# ── Step 0: Check CLI version ──"
Enter
Sleep 500ms

Type@0.03 "$ lattice --version"
Sleep 300ms
Enter
Sleep 400ms
Type@0 "lattice v0.3.0"
Enter
Enter
Sleep 1s

# ── Step 1: Start LatticeD (control plane) ──────────────────────────────

Type@0.05 "# ── Step 1: Start control plane ──"
Enter
Sleep 500ms

Type@0.03 "$ latticed start --dev"
Sleep 300ms
Enter
Sleep 600ms
Type@0 "INF Starting LatticeD all-in-one..."
Enter
Sleep 300ms
Type@0 "INF NATS server started on :4222"
Enter
Sleep 200ms
Type@0 "INF SQLite database initialized path=data/lattice.db"
Enter
Sleep 200ms
Type@0 "INF API server listening on :8080"
Enter
Sleep 200ms
Type@0 "INF Web UI available at http://localhost:8080"
Enter
Enter
Sleep 1s

# ── Step 2: Create workspace & tokens (via HTTP API) ────────────────────

Type@0.05 "# ── Step 2: Create workspace & enrollment token ──"
Enter
Sleep 500ms

Type@0.03 "$ curl -s -X POST http://localhost:8080/api/v1/users/login \\"
Enter
Type@0.03 "  -H 'Content-Type: application/json' \\"
Enter
Type@0.03 "  -d '{\"username\":\"admin\",\"password\":\"123456\"}'"
Sleep 300ms
Enter
Sleep 400ms
Type@0 '{"code":0,"data":{"token":"lt-admin-xxxxxxxxxxxx"}}'
Enter
Enter
Sleep 500ms

Type@0.03 '$ TOKEN="lt-admin-xxxxxxxxxxxx"'
Enter
Enter
Sleep 500ms

Type@0.03 "$ curl -s -X POST http://localhost:8080/api/v1/workspaces/add \\"
Enter
Type@0.03 '  -H "Authorization: Bearer $TOKEN" \'
Enter
Type@0.03 "  -H 'Content-Type: application/json' \\"
Enter
Type@0.03 "  -d '{\"namespace\":\"wf-e2e-demo\",\"displayName\":\"Sandbox Demo\",\"slug\":\"sandbox-demo\",\"maxNodeCount\":10}'"
Sleep 300ms
Enter
Sleep 400ms
Type@0 '{"code":0,"data":{"id":"ws-abc123","namespace":"wf-ws-abc123"}}'
Enter
Enter
Sleep 500ms

Type@0.03 "$ curl -s -X POST http://localhost:8080/api/v1/agent-isolation/enrollment-tokens \\"
Enter
Type@0.03 '  -H "Authorization: Bearer $TOKEN" \'
Enter
Type@0.03 "  -H 'Content-Type: application/json' \\"
Enter
Type@0.03 "  -d '{\"namespace\":\"wf-ws-abc123\",\"allowedTools\":[\"http\",\"exec\"],\"ttlSeconds\":600}'"
Sleep 300ms
Enter
Sleep 400ms
Type@0 '{"code":0,"data":{"token":"lt-enroll-xxxxxxxxxxxx"}}'
Enter
Enter
Sleep 1s

# ── Step 3: Companion agent joins the mesh ──────────────────────────────

Type@0.05 "# ── Step 3: Companion agent joins WireGuard mesh ──"
Enter
Sleep 500ms

Type@0.03 "$ lattice up \\"
Enter
Type@0.03 "  --token lt-join-xxxxxxxxxxxx \\"
Enter
Type@0.03 "  --server-url http://localhost:8080 \\"
Enter
Type@0.03 "  --level debug"
Sleep 300ms
Enter
Sleep 600ms
Type@0 "INF Connecting to NATS signaling server"
Enter
Sleep 300ms
Type@0 "INF WireGuard keypair generated"
Enter
Sleep 300ms
Type@0 "INF Peer enrolled peer=comp-001 overlayIP=10.42.0.1"
Enter
Sleep 500ms
Type@0 "INF ICE probing remote candidates..."
Enter
Sleep 400ms
Type@0 "INF ICE tunnel established peer=10.42.0.1"
Enter
Sleep 200ms
Type@0 "INF Tunnel status: READY"
Enter
Enter
Sleep 1s

# ── Step 4: Sandbox agent starts (gVisor zero-privilege) ────────────────

Type@0.05 "# ── Step 4: Sandbox agent starts (zero-privilege, gVisor) ──"
Enter
Sleep 500ms

Type@0.03 "$ lattice sandbox start \\"
Enter
Type@0.03 "  --name sandbox-001 \\"
Enter
Type@0.03 "  --server-url http://localhost:8080 \\"
Enter
Type@0.03 "  --token lt-enroll-xxxxxxxxxxxx \\"
Enter
Type@0.03 "  --proxy-addr 127.0.0.1:1080 \\"
Enter
Type@0.03 "  --forward 8080:127.0.0.1:8080"
Sleep 300ms
Enter
Sleep 600ms
Type@0 "INF Loading credentials path=/etc/lattice/sandbox-credentials.json"
Enter
Sleep 300ms
Type@0 "INF Registering sandbox via NATS name=sandbox-001"
Enter
Sleep 400ms
Type@0 "INF Sandbox credentials saved mode=0600"
Enter
Sleep 300ms
Type@0 "INF gVisor sandbox initialized id=sandbox-001 localIP=10.42.0.5"
Enter
Sleep 300ms
Type@0 "INF TUN adapter started"
Enter
Sleep 300ms
Type@0 "INF ForwardListener started overlay:8080 → 127.0.0.1:8080"
Enter
Sleep 200ms
Type@0 "INF HTTP proxy listening on 127.0.0.1:1080"
Enter
Sleep 400ms
Type@0 "INF ICE tunnel established peer=10.42.0.1"
Enter
Sleep 200ms
Type@0 "INF Tunnel status: READY"
Enter
Sleep 200ms
Type@0 "INF Node started, heartbeat every 30s"
Enter
Enter
Sleep 1s

# ── Scenario 1: AgentIdentity CRD ───────────────────────────────────────

Type@0.05 "# ── Scenario 1: AgentIdentity CRD created with Active phase ──"
Enter
Sleep 500ms

Type@0.03 "$ curl -s http://localhost:8080/api/v1/agent-isolation/agents/sandbox-001 \\"
Enter
Type@0.03 '  -H "Authorization: Bearer $TOKEN"'
Sleep 300ms
Enter
Sleep 400ms
Type@0 '{"code":0,"data":{"name":"sandbox-001","phase":"Active","vpnIP":"10.42.0.5"}}'
Enter
Sleep 300ms
Type@0 "# ✓ AgentIdentity exists, phase=Active"
Enter
Enter
Sleep 1s

# ── Scenario 2: Sandbox → Companion via WireGuard overlay ───────────────

Type@0.05 "# ── Scenario 2: Sandbox reaches companion nginx via WireGuard ──"
Enter
Sleep 500ms

Type@0.03 "$ http_proxy=http://127.0.0.1:1080 \\"
Enter
Type@0.03 "  wget -q -O - --timeout=5 http://10.42.0.1:8080"
Sleep 300ms
Enter
Sleep 500ms
Type@0 '<!DOCTYPE html>'
Enter
Type@0 '<html>'
Enter
Type@0 '<head><title>Welcome to nginx!</title></head>'
Enter
Type@0 '<body><h1>Welcome to nginx!</h1></body>'
Enter
Type@0 '</html>'
Enter
Sleep 300ms
Type@0 "# ✓ Sandbox reached companion via WireGuard overlay"
Enter
Enter
Sleep 1s

# ── Scenario 3: Companion → Sandbox via ForwardListener ─────────────────

Type@0.05 "# ── Scenario 3: Companion reaches sandbox nginx via ForwardListener ──"
Enter
Sleep 500ms

Type@0.03 "$ wget -q -O - --timeout=5 http://10.42.0.5:8080"
Sleep 300ms
Enter
Sleep 500ms
Type@0 '<!DOCTYPE html>'
Enter
Type@0 '<html>'
Enter
Type@0 '<head><title>Welcome to nginx!</title></head>'
Enter
Type@0 '<body><h1>Welcome to nginx!</h1></body>'
Enter
Type@0 '</html>'
Enter
Sleep 300ms
Type@0 "# ✓ Companion reached sandbox via ForwardListener"
Enter
Enter
Sleep 1s

# ── Scenario 4: Non-lattice IP BLOCKED ──────────────────────────────────

Type@0.05 "# ── Scenario 4: Non-lattice IP is BLOCKED by default ──"
Enter
Sleep 500ms

Type@0.03 "$ http_proxy=http://127.0.0.1:1080 \\"
Enter
Type@0.03 "  wget -q -O - --timeout=3 http://1.2.3.4:80"
Sleep 300ms
Enter
Sleep 1s
Type@0 "wget: download timed out"
Enter
Sleep 300ms
Type@0 "# ✓ Connection to non-lattice IP consistently blocked"
Enter
Enter
Sleep 1s

# ── Scenario 5: Policy DENY blocks companion ────────────────────────────

Type@0.05 "# ── Scenario 5: Policy DENY blocks sandbox → companion ──"
Enter
Sleep 500ms

Type@0.03 "$ curl -s -X PUT http://localhost:8080/api/v1/policies/e2e-allow-all \\"
Enter
Type@0.03 '  -H "Authorization: Bearer $TOKEN" \'
Enter
Type@0.03 "  -H 'Content-Type: application/json' \\"
Enter
Type@0.03 "  -d '{\"action\":\"DENY\"}'"
Sleep 300ms
Enter
Sleep 400ms
Type@0 '{"code":0,"data":{"name":"e2e-allow-all","action":"DENY"}}'
Enter
Sleep 500ms
Type@0 "# Policy change propagates to enforcer (iptables/eBPF)..."
Enter
Sleep 1s

Type@0.03 "$ http_proxy=http://127.0.0.1:1080 \\"
Enter
Type@0.03 "  wget -q -O - --timeout=3 http://10.42.0.1:8080"
Sleep 300ms
Enter
Sleep 1s
Type@0 "wget: download timed out"
Enter
Sleep 300ms
Type@0 "# ✓ DENY policy effective — companion unreachable"
Enter
Enter
Sleep 1s

# ── Scenario 6: Audit log shows allow + drop events ─────────────────────

Type@0.05 "# ── Scenario 6: Audit log captures allow & drop events ──"
Enter
Sleep 500ms

Type@0.03 "$ cat /tmp/lattice-audit-sandbox-001.jsonl"
Sleep 300ms
Enter
Sleep 500ms
Type@0 '{"timestamp":"2026-05-18T10:00:01Z","identity":"sandbox-001","src_ip":"10.42.0.5","dst_ip":"10.42.0.1","dst_port":8080,"protocol":"tcp","verdict":"allow"}'
Enter
Sleep 300ms
Type@0 '{"timestamp":"2026-05-18T10:00:05Z","identity":"sandbox-001","src_ip":"10.42.0.5","dst_ip":"1.2.3.4","dst_port":80,"protocol":"tcp","verdict":"drop"}'
Enter
Sleep 300ms
Type@0 '{"timestamp":"2026-05-18T10:00:10Z","identity":"sandbox-001","src_ip":"10.42.0.5","dst_ip":"10.42.0.1","dst_port":8080,"protocol":"tcp","verdict":"drop"}'
Enter
Sleep 300ms
Type@0 "# ✓ Audit log has allow events (scenario 2) and drop events (scenarios 4,5)"
Enter
Enter
Sleep 1s

# ── Scenario 7: Agent revocation ────────────────────────────────────────

Type@0.05 "# ── Scenario 7: Agent revocation stops connections ──"
Enter
Sleep 500ms

Type@0.03 "$ curl -s -X DELETE \\"
Enter
Type@0.03 "  http://localhost:8080/api/v1/agent-isolation/agents/sandbox-001 \\"
Enter
Type@0.03 '  -H "Authorization: Bearer $TOKEN"'
Sleep 300ms
Enter
Sleep 400ms
Type@0 '{"code":0,"msg":"agent revoked"}'
Enter
Enter
Sleep 500ms

Type@0.03 "$ curl -s http://localhost:8080/api/v1/agent-isolation/agents/sandbox-001 \\"
Enter
Type@0.03 '  -H "Authorization: Bearer $TOKEN"'
Sleep 300ms
Enter
Sleep 400ms
Type@0 '{"code":0,"data":{"name":"sandbox-001","phase":"Revoked","vpnIP":"10.42.0.5"}}'
Enter
Sleep 300ms
Type@0 "# ✓ AgentIdentity phase=Revoked — connections permanently stopped"
Enter
Enter
Sleep 1s

# ── Summary ──────────────────────────────────────────────────────────────

Type@0.05 "# ═══════════════════════════════════════════════"
Enter
Type@0.05 "#  7/7 scenarios passed"
Enter
Type@0.05 "#  ✓ WireGuard overlay connectivity (both ways)"
Enter
Type@0.05 "#  ✓ Default egress block (non-lattice IP)"
Enter
Type@0.05 "#  ✓ Policy DENY enforcement"
Enter
Type@0.05 "#  ✓ Audit logging (allow + drop events)"
Enter
Type@0.05 "#  ✓ Agent revocation"
Enter
Type@0.05 "# ═══════════════════════════════════════════════"
Enter
Sleep 2s
