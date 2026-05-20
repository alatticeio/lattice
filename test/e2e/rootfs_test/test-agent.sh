#!/bin/sh
# Minimal test agent for runsc E2E tests.
# Runs as PID 1 inside the gVisor container. The Lattice overlay (wg0) is
# already set up on the pod kernel before runsc starts (Phase 1).
# Keeps the container alive so we can runsc exec in and test connectivity.
echo "[test-agent] Running inside gVisor container"
exec sleep infinity
