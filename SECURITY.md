# Security Policy

## Reporting a Vulnerability

We take the security of Lattice seriously. If you discover a security vulnerability, please report it privately **before** creating a public issue.

**Do not report security vulnerabilities through public GitHub issues.**

Instead, report via email to: **info@lattice.run**

You should receive a response within 48 hours. If you do not, please follow up to ensure we received your original message.

When reporting, please include:

- A description of the vulnerability
- Steps to reproduce (if applicable)
- Affected versions
- Any potential impact or exploit scenarios

## What to Expect

- **Acknowledgement** — We'll confirm receipt within 48 hours
- **Triage** — We'll assess severity and impact within 5 business days
- **Fix** — We'll develop and release a fix based on severity
- **Disclosure** — We'll coordinate public disclosure timing with you

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest stable release | ✅ |
| older releases | ❌ |

We recommend always using the latest stable release of Lattice.

## Scope

Security issues in:

- The `lattice` agent binary
- The `latticed` all-in-one control plane
- The `manager` Kubernetes operator
- The web dashboard
- The `lrper` relay server

Out of scope:

- Security of third-party dependencies (report to their maintainers)
- Issues requiring physical access to a device
- Social engineering attacks

## Recognition

We believe in acknowledging security researchers who help keep Lattice safe.
With your permission, we'll credit you in the release notes for any verified
security fix.

---

**Thank you for helping keep Lattice and its users secure.**
