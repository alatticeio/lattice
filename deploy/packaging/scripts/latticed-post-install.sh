#!/bin/sh
set -e
systemctl daemon-reload || true
systemctl enable latticed.service || true
echo "Latticed installed successfully."
echo "Start: systemctl start latticed"
echo "Status: systemctl status latticed"
