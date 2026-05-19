#!/bin/sh
# Minimal test agent for runsc E2E tests.
# Keeps the container alive so we can exec in and test overlay connectivity.
# The lattice sandbox agent execs this after setting up the WireGuard overlay.
echo "[test-agent] Running inside gVisor container, overlay ready"
exec sleep infinity
