# lattice/installer — zero-dependency bootstrapper image
# Bundles k3d + kubectl so the user only needs Docker.
#
# Build (multi-arch):
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -t lattice/installer:latest \
#     --push deploy/installer/
#
# Run:
#   docker run --rm -it \
#     -v /var/run/docker.sock:/var/run/docker.sock \
#     -v ~/.kube:/root/.kube \
#     lattice/installer:latest
#
# Cleanup:
#   docker run --rm \
#     -v /var/run/docker.sock:/var/run/docker.sock \
#     -e CLEANUP=true \
#     lattice/installer:latest

FROM alpine:3.20

ARG TARGETOS=linux
ARG TARGETARCH

# Install runtime deps
RUN apk add --no-cache \
    bash \
    curl \
    ca-certificates \
    jq \
    procps \
    docker-cli

# Install k3d (single self-contained binary)
RUN K3D_VERSION="v5.7.3" && \
    curl -fsSL "https://github.com/k3d-io/k3d/releases/download/${K3D_VERSION}/k3d-${TARGETOS}-${TARGETARCH}" \
      -o /usr/local/bin/k3d && \
    chmod +x /usr/local/bin/k3d

# Install kubectl
RUN K8S_VERSION="v1.30.3" && \
    curl -fsSL "https://dl.k8s.io/release/${K8S_VERSION}/bin/${TARGETOS}/${TARGETARCH}/kubectl" \
      -o /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/kubectl

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
