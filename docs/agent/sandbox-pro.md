---
title: Agent Sandbox (Pro)
---

# Agent Sandbox — Pro Edition

The Pro sandbox extends the [Community sandbox](/agent/sandbox) with egress filtering, inbound port forwarding, a SOCKS5 proxy, and server-side NATS flow auditing.

## Additional Flags

```
lattice sandbox start [flags]

Pro-only flags:
  --egress-allow   strings   CIDR ranges to allow outbound (default: deny-all)
  --forward        strings   Overlay port → host address mappings (e.g. 8080:127.0.0.1:8080)
  --proxy-addr     string    Local address for SOCKS5 proxy (e.g. 127.0.0.1:1080)
```

## EgressFilter

EgressFilter implements `PolicyChecker` — it intercepts every outbound connection attempt from the gVisor netstack and evaluates it against a CIDR allowlist.

```bash
# Only allow outbound to the tool service at 10.42.0.10 and HTTPS
lattice sandbox start \
  --name agent-001 \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --egress-allow 10.42.0.10/32 \
  --egress-allow 0.0.0.0/0:443
```

If no `--egress-allow` flags are provided, all egress is denied. Denied connections are logged to the audit file.

## ForwardListener

Forward inbound connections from the overlay network to a host-local address:

```bash
# Accept connections on overlay port 8080, forward to localhost:8080
lattice sandbox start \
  --name api-agent \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --forward 8080:127.0.0.1:8080
```

Multiple `--forward` flags are supported.

## SOCKS5 Proxy

Start a SOCKS5 proxy inside the gVisor sandbox. All TCP connections through this
proxy are routed through the WireGuard overlay with full policy enforcement and
audit logging.

```bash
lattice sandbox start \
  --name browser-agent \
  --server-url http://lattice.internal:8080 \
  --token lt-xxx \
  --proxy-addr 127.0.0.1:1080
```

In your agent process:

```bash
export ALL_PROXY=socks5://127.0.0.1:1080
curl https://internal-api.example.com/data
```

## NATS Flow Audit (Server-side)

Pro adds server-side persistence of network flow events. The sandbox publishes to `lattice.audit.flow` on NATS; the control plane's `AuditConsumer` persists events to the `la_flow_events` database table.

> **Status:** Server-side pipeline is complete (`AuditConsumer` + `la_flow_events`). The sandbox-side `natsAuditWriter` is in development — sandbox currently writes to local file only.

Query audit events via the dashboard or API:

```
GET /api/v1/audit/flow?agentName=agent-001&from=2026-05-18T00:00:00Z
```
