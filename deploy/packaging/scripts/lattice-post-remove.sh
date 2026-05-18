#!/bin/sh
set -e
# Stop and disable
systemctl stop lattice.service || true
systemctl disable lattice.service || true
# Reload systemd
systemctl daemon-reload || true
