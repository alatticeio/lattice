#!/bin/sh
set -e
systemctl stop latticed.service || true
systemctl disable latticed.service || true
systemctl daemon-reload || true
