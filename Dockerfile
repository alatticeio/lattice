# Build the manager binary
FROM registry.cn-hangzhou.aliyuncs.com/wireflow-io/golang:1.25.2 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG TARGETSERVICE

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
# RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download

# Copy the go source
COPY . . 




# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o $TARGETSERVICE ./cmd/$TARGETSERVICE/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
FROM alpine:latest
# 安装必要的网络工具，方便调试权限
RUN apk add --no-cache wireguard-tools iptables iproute2
ARG TARGETSERVICE

RUN mkdir -p /app /etc/wireflow /var/log/wireflow

WORKDIR /app
ENV WIREFLOW_CONFIG_DIR=/etc/wireflow
ENV HOME=/app
COPY --from=builder /workspace/$TARGETSERVICE .//wireflow

ENTRYPOINT ["/app/wireflow"]
