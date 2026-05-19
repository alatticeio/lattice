# Rootfs image for runsc gVisor E2E tests.
# This image is used as the container filesystem when running
# `lattice sandbox start --mode gvisor --agent-rootfs <this-rootfs>`.
#
# The lattice binary is placed at /usr/local/bin/lattice so that the
# runsc OCI spec (which sets process.args = ["lattice", "sandbox", "agent", ...])
# can find it. The test agent binary (/usr/local/bin/test-agent) keeps the
# container alive after the overlay network is set up.
#
# Build:
#   cd test/e2e && docker build -t lattice-runsc-rootfs:e2e -f rootfs_test/Dockerfile ../../
# Export rootfs:
#   docker export $(docker create lattice-runsc-rootfs:e2e) -o /tmp/lattice-runsc-rootfs.tar
#   mkdir -p /tmp/lattice-runsc-rootfs && tar -xf /tmp/lattice-runsc-rootfs.tar -C /tmp/lattice-runsc-rootfs
#
# Then run the E2E test with --agent-rootfs /tmp/lattice-runsc-rootfs --agent-binary /usr/local/bin/test-agent

FROM alpine:3.21

# Copy the lattice binary (built by make build SERVICE=lattice).
# The Docker context is the repository root (test/e2e/../../).
COPY bin/lattice /usr/local/bin/lattice

# Simple test agent: keeps the container alive so we can exec into it
# and test overlay connectivity.
COPY test/e2e/rootfs_test/test-agent.sh /usr/local/bin/test-agent
RUN chmod +x /usr/local/bin/test-agent

# Verify binaries exist.
RUN /usr/local/bin/lattice --version 2>&1 || true
RUN test -x /usr/local/bin/test-agent

ENTRYPOINT ["/usr/local/bin/test-agent"]
