# syntax=docker/dockerfile:1
# Lattice K3s — All-in-One: K8s cluster + Lattice control plane in a single container.
#
# Build:
#   docker build -f deploy/k3s/Dockerfile -t lattice-k3s:latest .
#
# Run:
#   docker run -d --privileged -p 8080:8080 -p 4222:4222 lattice-k3s:latest
#
# Environment variables:
#   LATTICE_ADMIN_USER     admin username (default: admin)
#   LATTICE_ADMIN_PASS     admin password (default: changeme)
#   LATTICE_JWT_SECRET     JWT signing secret (default: auto-generated)
#   LATTICE_DATA_DIR       SQLite data directory (default: /app/data)
#   LATTICE_CONFIG_DIR     config directory (default: /etc/lattice)

# ── Stage 1: Build latticed binary ──────────────────────────────────────────
FROM golang:1.25.2 AS builder

ARG BUILD_TAGS=""

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux \
    ${BUILD_TAGS:+-tags ${BUILD_TAGS}} \
    go build -o /latticed ./cmd/latticed/

# ── Stage 2: k3s with embedded latticed ─────────────────────────────────────
FROM rancher/k3s:latest

# Copy latticed binary
COPY --from=builder /latticed /usr/bin/latticed

# Copy CRDs to k3s auto-deploy directory (applied automatically at startup)
COPY config/crd/bases/ /var/lib/rancher/k3s/server/manifests/crds/

# Copy entrypoint
COPY deploy/k3s/start.sh /start.sh
RUN chmod +x /start.sh

EXPOSE 8080 4222

VOLUME ["/app/data"]

# Clear ENTRYPOINT from parent image (rancher/k3s uses k3s as entrypoint)
ENTRYPOINT []
CMD ["/start.sh"]
