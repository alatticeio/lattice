# Lattice AI Agent Sandbox
# Build: docker build -f deploy/docker/sandbox.Dockerfile --build-arg BUILD_TAGS=pro -t lattice-sandbox .
#
# This image packages `lattice sandbox start` as a standalone sidecar.
# It requires no kernel modules or privileges — gVisor runs entirely in userspace.
#
# Usage: see deploy/docker/docker-compose.sandbox.yml

FROM golang:1.25.8 AS builder
ARG BUILD_TAGS="pro"

WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build ${BUILD_TAGS:+-tags ${BUILD_TAGS}} -o lattice ./cmd/lattice/main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates curl

RUN mkdir -p /etc/lattice /tmp
WORKDIR /app

COPY --from=builder /workspace/lattice /app/lattice
COPY deploy/docker/sandbox-entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/lattice /app/entrypoint.sh

# Default env vars (override at runtime)
ENV LATTICE_CONFIG_DIR=/etc/lattice
ENV PROXY_ADDR=127.0.0.1:1080
ENV EGRESS_DEFAULT_DENY=true

ENTRYPOINT ["/app/entrypoint.sh"]
