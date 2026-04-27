#!/bin/sh
set -e
cd "$(dirname "$0")"
podman build -t speconn-go:build -f Containerfile.build .
echo "speconn-go: build OK"
