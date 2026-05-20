# Rootfs image for runsc gVisor E2E tests.
# This image is used as the container filesystem when running
# `lattice sandbox start --mode gvisor --agent-rootfs <this-rootfs>`.
#
# Architecture (two-phase):
#   Phase 1 (pod kernel): NATS registration + WireGuard (wg0) on real kernel.
#   Phase 2 (gVisor): OCI spec runs test-agent directly as PID 1 via
#     --network=host, inheriting the pod's pre-configured network namespace.
#
# The lattice binary is NOT needed inside the container — all Lattice
# infrastructure runs on the pod kernel (Phase 1).
#
# curl is included so the E2E test can verify overlay connectivity:
#   runsc exec <sandbox-id> curl -s http://<peer-ip>:8080
#
# Build:
#   cd test/e2e && docker build -t lattice-runsc-rootfs:e2e -f rootfs_test/Dockerfile ../../
# Export rootfs:
#   docker export $(docker create lattice-runsc-rootfs:e2e) -o /tmp/lattice-runsc-rootfs.tar
#   mkdir -p /tmp/lattice-runsc-rootfs && tar -xf /tmp/lattice-runsc-rootfs.tar -C /tmp/lattice-runsc-rootfs
#
# Then run the E2E test with --agent-rootfs /tmp/lattice-runsc-rootfs --agent-binary /usr/local/bin/test-agent

FROM alpine:3.21

# curl needed for E2E overlay connectivity verification (runsc exec).
RUN apk add --no-cache curl

# Simple test agent: keeps the container alive so we can exec into it
# and test overlay connectivity. Runs as PID 1 inside runsc.
COPY test/e2e/rootfs_test/test-agent.sh /usr/local/bin/test-agent
RUN chmod +x /usr/local/bin/test-agent

# Verify binaries exist.
RUN test -x /usr/local/bin/test-agent

ENTRYPOINT ["/usr/local/bin/test-agent"]
