# ICE Candidate Race: P2P Handshake Timeout Fix

## Background

In the agent-sandbox E2E test, the WireGuard tunnel between a companion pod and a sandbox pod
consistently failed to establish within the 180-second timeout. The symptom was `wget: download
timed out`, caused by the ICE handshake never completing and therefore the WireGuard endpoint
never being set.

## Root Cause Analysis

The failure was caused by **two independent bugs** that compounded each other.

### Bug 1 — Missing `AppId` in sandbox LatticePeer (primary cause)

`internal/server/service/agent_registration.go` creates the LatticePeer CRD when a sandbox agent
enrolls. The original code did **not** set `Spec.AppId`:

```go
// Before
Spec: v1alpha1.LatticePeerSpec{
    PublicKey: req.PublicKey,
    Network:   networkRef,
    // AppId omitted
},
```

`LatticePeerSpec.AppId` carries `json:"appId,omitempty"`, so a zero-value `""` is silently dropped
when the controller serialises the peer list into the ConfigMap. The companion receives a
`computedpeers` entry with no `appId` field:

```json
{"name":"sandbox-xxx","address":"10.0.5.3","publicKey":"RWcG...","peerId":5000973369834774145,...}
```

### Bug 2 — ICE candidates arriving before the agent is initialised (secondary cause)

ICE signalling is asymmetric: the non-initiator (sandbox) may send OFFER candidates before the
initiator (companion) has finished constructing its `ice.Agent` inside `Prepare()`. The original
`Handle()` returned early when `i.agent == nil`, silently discarding those candidates.

### How the two bugs interact

1. Companion receives `appId=""` for the sandbox → `peerManager.AddPeer("", peer)` → probe **P1**
   created with key `""` → `Prepare()` builds ICE agent **A1**, starts gathering.
2. Sandbox sends ACK carrying `appId="sandbox-xxx"` → `onPeerReceived` updates `byID` to point to
   the peer with the correct AppID.
3. Sandbox sends ICE OFFER → `ProbeFactory.Handle()` calls `GetIdentity(peerID)` → resolves to
   `AppID="sandbox-xxx"` → `Get("sandbox-xxx")` → **not found** → creates new probe **P2**
   (no ICE agent).
4. P2 receives the OFFER but has `agent == nil` — candidate is buffered but never processed.
5. P1's `Dial()` blocks on `offerReady` channel which never closes → 65-second timeout.
6. After the 60-second unreachability threshold, the probe is closed → E2E test fails.

The log evidence confirmed the sequence:

```
peer known, pre-configured WG entry  remoteId=""              allowedIPs=10.0.5.3/32  (P1 created)
peer known, pre-configured WG entry  remoteId=sandbox-xxx     allowedIPs=10.0.5.3/32  (P2 created)
receive offer: agent not ready, buffering candidate                                    (P2, no agent)
iceDialer: timed out waiting for offer                                                 (P1 times out)
peer unreachable for 60s, closing probe
```

## Fixes

### Fix 1 — Set `AppId` at enrollment (`agent_registration.go`)

```go
Spec: v1alpha1.LatticePeerSpec{
    AppId:     req.AgentName,   // always set so ConfigMap JSON includes appId
    PublicKey: req.PublicKey,
    Network:   networkRef,
},
```

This ensures the ConfigMap always serialises a non-empty `appId` matching the peer `name`,
eliminating the probe key mismatch entirely.

### Fix 2 — Defensive fallback in `transferToPeer` (`controller/utils.go`)

```go
appID := peer.Spec.AppId
if appID == "" {
    appID = peer.Name
}
```

Guards against any existing or future LatticePeer that reaches the controller without `Spec.AppId`
set (e.g. hand-crafted CRDs, older manifests).

### Fix 3 — Buffer and replay ICE candidates (`ice_dialer.go`)

- `Handle(OFFER)` appends candidates to `pendingCandidates` when `agent == nil` instead of
  dropping them.
- `Prepare()` drains `pendingCandidates` into the newly created agent after `i.agent` is set.
- `GatherCandidates` is guarded with a `sync.Once` (`gatherOnce`) so it is called exactly once
  even when an ACK races ahead of the agent initialisation and re-triggers the gather path.

## Affected Files

| File | Change |
|------|--------|
| `internal/server/service/agent_registration.go` | Set `Spec.AppId = req.AgentName` |
| `internal/agent/controller/utils.go` | Fallback `AppId → Name` in `transferToPeer` |
| `internal/server/transport/ice_dialer.go` | Buffer/replay candidates; `sync.Once` for `GatherCandidates` |

## Consequences

- Sandbox peers now always appear with a stable, non-empty `appId` in the network ConfigMap.
- ICE candidates sent before the local agent is ready are no longer silently dropped; they are
  replayed immediately after `Prepare()` completes.
- The `ProbeFactory` will never create a duplicate probe for the same physical peer due to an
  `appId`/`name` mismatch.
