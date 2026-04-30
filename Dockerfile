# Build the manager binary
FROM golang:1.25.2 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETSERVICE
# BUILD_TAGS: set to "pro" for Pro edition, empty for community
ARG BUILD_TAGS=""

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.sum ./

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY . . 




# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build \
    ${BUILD_TAGS:+-tags ${BUILD_TAGS}} \
    -o $TARGETSERVICE ./cmd/$TARGETSERVICE/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
FROM alpine:latest
ARG TARGETSERVICE

# 根据服务名称动态安装依赖：
#   lattice  (edge agent)  -> 需要 WireGuard / iptables / iproute2
#   latticed (all-in-one)  -> 同上，另需 ca-certificates（HTTPS 出向请求）
#   manager   (K8s operator) -> 仅需 ca-certificates
RUN if [ "$TARGETSERVICE" = "lattice" ] || [ "$TARGETSERVICE" = "latticed" ]; then \
        apk add --no-cache wireguard-tools iptables iproute2 ca-certificates; \
    else \
        apk add --no-cache ca-certificates; \
    fi

RUN mkdir -p /app /etc/lattice /var/log/lattice

WORKDIR /app
ENV LATTICE_CONFIG_DIR=/etc/lattice
ENV HOME=/app
COPY --from=builder /workspace/$TARGETSERVICE ./lattice

ENTRYPOINT ["/app/lattice"]
