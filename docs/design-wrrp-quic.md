# WRRP QUIC Transport Design

## 1. Background

WRRP (Wireflow Relay & Routing Protocol) is the relay channel used when two peers cannot establish a direct ICE path (e.g. symmetric NAT on both sides). The original implementation tunnels WireGuard packets over a persistent HTTP-upgraded TCP connection.

### Problems with TCP relay

| Problem | Impact |
|---|---|
| Head-of-Line (HoL) blocking | A dropped TCP segment stalls all peers multiplexed on the same connection |
| Application-level keepalive | Custom Ping/Pong logic required to detect dead connections |
| Slow reconnect | TCP + HTTP handshake on every reconnect |
| No packet-boundary preservation | Framing layer required on top of TCP stream |

## 2. Goals

1. **Eliminate HoL blocking** between peer pairs sharing a relay connection.
2. **Simplify keepalive** — remove app-level Ping frames; delegate to QUIC's built-in mechanism.
3. **Faster reconnect** — QUIC 0-RTT resumption reduces reconnect latency.
4. **Preserve packet boundaries** — QUIC datagrams map 1:1 to WireGuard packets.
5. **Backward compatibility** — TCP relay path remains fully operational; QUIC is opt-in.

## 3. Non-Goals

- **P2P QUIC between peers** — QUIC cannot replace ICE for NAT traversal. ICE handles hole-punching (STUN/TURN); QUIC cannot do this. ICE remains the primary path for direct connectivity.
- **gVisor netstack** — Not adopted; ~20-40% lower throughput vs kernel TUN with no benefit for server/macOS deployments.
- **TLS PKI** — The relay server generates a self-signed certificate at startup. Clients use `InsecureSkipVerify`. The underlying WireGuard encryption provides end-to-end security; relay TLS only protects the outer transport.

## 4. Architecture

```
┌──────────────────────────────────────────────────────────┐
│                    WRRP Relay Server                     │
│                                                          │
│  ┌─────────────────────┐   ┌─────────────────────────┐  │
│  │   TCP Server        │   │   QUIC Server           │  │
│  │   :6266 (HTTP upg.) │   │   :6267 (UDP)           │  │
│  └────────┬────────────┘   └──────────┬──────────────┘  │
│           │                           │                  │
│           └──────────┐   ┌────────────┘                  │
│                      ▼   ▼                               │
│               ┌─────────────────┐                        │
│               │  WRRPManager    │                        │
│               │  streams map    │  ← TCP sessions        │
│               │  quicConns map  │  ← QUIC sessions       │
│               └────────┬────────┘                        │
│                        │ Relay(toID, frame)              │
│                        │  • QUIC: SendDatagram           │
│                        │  • TCP:  Stream.Write           │
└────────────────────────┼─────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         ▼                               ▼
  ┌─────────────┐                 ┌─────────────┐
  │  Agent A    │                 │  Agent B    │
  │  QUIC client│                 │  QUIC client│
  └─────────────┘                 └─────────────┘
```

### Control stream vs data datagrams

```
Client                              Server
  │                                    │
  │──── QUIC handshake (TLS) ─────────►│
  │◄─── handshake complete ────────────│
  │                                    │
  │──── OpenStreamSync (stream 0) ────►│  control stream
  │──── Register header (28 bytes) ───►│  identify this client
  │                                    │
  │══════════ QUIC datagrams ══════════│  Forward / Probe packets
  │                                    │  (unreliable, no HoL blocking)
  │◄══════════ QUIC datagrams ═════════│
  │                                    │
  │  [QUIC KeepAlivePeriod=25s]        │  replaces app-level Ping
  │  [MaxIdleTimeout=90s]              │  connection liveness
```

## 5. Protocol Details

### WRRP Header (28 bytes, unchanged)

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
├─────────────────────────────────────────────────────────────────┤
│                         Magic (4)                               │
├─────────────┬───────────────┬──────────────────────────────────┤
│  Version(1) │    Cmd (1)    │         Reserved (2)             │
├─────────────────────────────────────────────────────────────────┤
│                       PayloadLen (4)                            │
├─────────────────────────────────────────────────────────────────┤
│                        FromID (8)                               │
├─────────────────────────────────────────────────────────────────┤
│                         ToID (8)                                │
└─────────────────────────────────────────────────────────────────┘
```

Commands:
- `0x01` Register — sent on control stream at session start
- `0x02` Forward  — WireGuard encrypted datagram (relay payload)
- `0x03` Probe    — ICE signaling packet (probe payload)
- `0x04` Ping     — keepalive (TCP only; QUIC uses transport-level keepalive)

### QUIC datagram frame layout

```
[ WRRP Header (28 bytes) | WireGuard/Probe payload ]
```

The entire frame is sent as a single QUIC datagram. This maps perfectly to WireGuard's packet model (fixed-size encrypted packets, MTU=1280).

## 6. Key Design Decisions

### 6.1 Datagrams for Forward/Probe, stream for registration only

QUIC streams provide reliability and ordering (like TCP). Using streams for WireGuard relay would reintroduce HoL blocking between streams on the same connection and add unnecessary retransmit overhead for encrypted packets (WireGuard handles its own replay protection).

QUIC datagrams are unreliable and unordered — exactly what WireGuard expects from its underlying transport. If a datagram is lost, WireGuard's own retransmit handles it.

The control stream carries only the initial `Register` header. After registration, it remains open purely to anchor the QUIC connection lifetime and to allow future control messages (e.g. graceful disconnect).

### 6.2 WRRPManager unified Relay()

Both TCP and QUIC sessions are registered in the same `WRRPManager`:

```go
func (w *WRRPManager) Relay(toID uint64, frame []byte) error {
    w.mu.Lock()
    qconn   := w.quicConns[toID]   // prefer QUIC
    session := w.streams[toID]     // fallback to TCP
    w.mu.Unlock()

    if qconn != nil {
        return qconn.SendDatagram(frame)
    }
    if session != nil {
        _, err := session.Stream.Write(frame)
        return err
    }
    return fmt.Errorf("relay target not found: %d", toID)
}
```

This allows a mixed deployment: some peers connect via QUIC, others via TCP; the relay server handles both transparently.

### 6.3 Self-signed TLS

The relay server generates a self-signed RSA-2048 certificate valid for 10 years at startup. Clients connect with `InsecureSkipVerify: true`.

This is acceptable because:
- All WireGuard traffic is end-to-end encrypted. The relay server sees only ciphertext.
- QUIC TLS is only needed to satisfy the QUIC protocol requirement for encryption. It does not add meaningful security to already-encrypted WireGuard packets.
- A full PKI would add operational complexity for a component whose security model relies entirely on WireGuard keys.

### 6.4 No P2P QUIC

ICE performs UDP hole-punching by sending STUN connectivity-check packets from both sides simultaneously. This works at the UDP layer regardless of what protocol runs on top.

Running QUIC over an ICE-discovered path would require:
1. Implementing `conn.Read`/`conn.Write` on `ICETransport` (currently stubs).
2. A custom `net.PacketConn` adapter wrapping the ICE transport for `quic.Dial`.
3. QUIC connection migration support to survive ICE path changes (e.g. network switch).

The ROI is low: the ICE direct path is already low-latency kernel UDP. QUIC adds TLS overhead and ~10-15% throughput reduction vs raw UDP for an already-optimal path. QUIC's main benefits (0-RTT reconnect, connection migration) only apply meaningfully for the relay path.

## 7. File Structure

```
wrrper/
├── server.go           # WRRPManager, TCP HTTP-upgrade server
├── server_quic.go      # QUICServer, quicControlStream, GenerateSelfSignedTLS
├── client.go           # TCP WRRPClient (existing)
├── client_quic.go      # QUICWRRPClient implementing infra.Wrrp
└── task.go             # Task struct (probe worker queue item)

cmd/wrrper/
└── main.go             # Standalone relay server binary

cmd/manager/cmd/
└── wrrp.go             # wrrper subcommand of manager (existing)
```

## 8. Configuration

### Server (`manager wrrper` or standalone `wrrper`)

| Flag | Default | Description |
|---|---|---|
| `--listen` / `-l` | `:6266` | TCP WRRP listen address |
| `--enable-tls` | `false` | Enable TLS on TCP listener |
| `--wrrp-quic-url` | `""` | QUIC listen address; empty disables QUIC |
| `--level` | `info` | Log level |

### Agent (`wireflow up`)

| Flag | Default | Description |
|---|---|---|
| `--enable-wrrp` | `false` | Enable WRRP relay fallback |
| `--wrrp-url` | `""` | TCP relay server address |
| `--wrrp-quic-url` | `""` | QUIC relay server address; takes priority over `--wrrp-url` |
| `--wg-port` | `51820` | WireGuard/ICE UDP port |

## 9. Deployment

### Standalone relay server

```bash
# TCP only
wrrper --listen :6266

# TCP + QUIC
wrrper --listen :6266 --wrrp-quic-url :6267

# Docker
docker run -p 6266:6266/tcp -p 6267:6267/udp \
  ghcr.io/wireflowio/wrrper:latest \
  --listen :6266 --wrrp-quic-url :6267
```

### Agent connecting via QUIC

```bash
wireflow up --token <token> \
  --enable-wrrp \
  --wrrp-quic-url <relay_ip>:6267
```

### Kubernetes (as sidecar to wireflowd or standalone Deployment)

```yaml
# Standalone wrrper Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wrrper
spec:
  template:
    spec:
      containers:
      - name: wrrper
        image: ghcr.io/wireflowio/wrrper:latest
        args:
        - --listen=:6266
        - --wrrp-quic-url=:6267
        ports:
        - containerPort: 6266   # TCP
          protocol: TCP
        - containerPort: 6267   # QUIC (UDP)
          protocol: UDP
```

## 10. Performance Characteristics

| Metric | TCP relay | QUIC relay |
|---|---|---|
| HoL blocking | Yes (per connection) | No (datagrams are independent) |
| Reconnect latency | TCP 3-way + HTTP upgrade | QUIC 0-RTT (resumed session) |
| Keepalive | Application Ping frame | Transport-level (KeepAlivePeriod) |
| Packet boundaries | Framing required | Native (datagram = packet) |
| CPU overhead | Low | Slightly higher (TLS per datagram) |
| MTU | Unlimited (stream) | ~1200 bytes per datagram (fits WG 1280 MTU) |
| Packet loss handling | TCP retransmit (harmful for WG) | Drop (WG handles retry) |

## 11. Future Work

- **TLS certificate provisioning** — support loading a real cert/key pair via `--tls-cert`/`--tls-key` flags for production deployments.
- **QUIC connection migration** — relevant for mobile agents that switch networks (WiFi → LTE). Requires agent-side handling.
- **Metrics** — expose per-peer relay byte counters and datagram drop rates via the existing VictoriaMetrics endpoint.
- **Rate limiting** — per-peer datagram rate limiting on the relay server to prevent abuse.
