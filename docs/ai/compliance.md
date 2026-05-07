---
title: Compliance-as-Conversation
---

# Compliance-as-Conversation (Pro)

Generate compliance reports and downloadable evidence packages from your Lattice network state — as easily as asking a question. Designed for SOC2 Type II, PCI-DSS, and HIPAA audits.

> **Pro feature.** Available in the Pro edition.

## Supported Frameworks

| Framework | Status |
|-----------|--------|
| SOC2 Type II | Supported |
| PCI-DSS | Supported (MVP) |
| HIPAA | Supported |

## How It Works

Lattice maps your network configuration to compliance controls automatically:

| Data Source | Compliance Use |
|-------------|----------------|
| LatticePolicy CRDs | Verify isolation rules exist and are correct |
| NetworkSnapshot history | Verify no unauthorized changes during audit period |
| WorkflowService approval records | Verify every change has a reviewedBy |

### PCI-DSS Mapping Example

| PCI-DSS Requirement | Lattice Check |
|--------------------|--------------|
| 1.3 — No direct public access to cardholder data | Verify `cardholder-data` peer has no ALLOW ingress from `*` |
| 1.2 — Restrict inbound and outbound traffic | Verify deny-all base policy exists |
| 10.2 — Log all access events | Verify every policy change has `reviewedBy` field |
| 6.4 — Change management process | Verify no `auto_approve` change records exist |

## API

### Assess — Run Compliance Check

```http
POST /api/v1/ai/compliance/assess
```

```json
{
  "workspace_id": "ws-prod",
  "framework": "pci-dss",
  "period_start": "2026-01-01T00:00:00Z",
  "period_end": "2026-05-06T00:00:00Z"
}
```

Response:

```json
{
  "framework": "pci-dss",
  "generated_at": "2026-05-06T12:00:00Z",
  "score": 92,
  "status": "partial",
  "controls": [
    {
      "id": "PCI-DSS-1.3",
      "title": "No direct public access to cardholder data",
      "status": "pass",
      "evidence": "cardholder-data peer has ingress restricted to internal CIDRs only",
      "remediation": ""
    },
    {
      "id": "PCI-DSS-1.2",
      "title": "Restrict inbound and outbound traffic",
      "status": "fail",
      "evidence": "No default-deny base policy found",
      "remediation": "Create a LatticePolicy that denies all ingress/egress by default"
    }
  ],
  "summary": "2 of 4 PCI-DSS controls pass. Missing default-deny policy is the key gap.",
  "evidence_id": "ev-20260506"
}
```

### Generate Evidence Package

```http
POST /api/v1/ai/compliance/evidence
```

Returns a downloadable ZIP archive:

```
lattice-compliance-evidence-2026-05-06.zip
+-- executive-summary.md
+-- controls/
|   +-- PCI-DSS-1.2-result.md
|   +-- PCI-DSS-1.3-result.md
|   +-- ...
+-- raw-data/
|   +-- policies-current.yaml
|   +-- snapshots-timeline.json
|   +-- workflow-audit-log.csv
+-- attestation.json    # SHA256 tamper-proof signature
```

## Pricing

| Feature | Community | Pro |
|---------|-----------|-----|
| Basic security audit (existing Audit) | Yes | Yes |
| Compliance framework assessment | 402 | Yes |
| Evidence package (ZIP) | 402 | Yes |
| Custom compliance frameworks | No | Yes |
| Audit report history | No | Yes (3 years) |
