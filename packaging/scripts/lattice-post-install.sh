#!/bin/sh
set -e
# Reload systemd unit files
systemctl daemon-reload || true
# Enable but do NOT start (user must configure first)
systemctl enable lattice.service || true
echo "Lattice installed successfully."
echo "Configure: /etc/lattice/config.yaml"
echo "Start: systemctl start lattice"
echo "Status: systemctl status lattice"
