#!/usr/bin/env bash
# Build prod releases using Docker (dobby context).
# Usage: bash build.sh
# Requires: Docker context "dobby" (or change DOCKER_CONTEXT below)
#
# For macOS build you still need a Mac or GitHub Actions (Apple doesn't
# allow cross-compilation of CGO apps for macOS).

set -e

OUT="dist"
mkdir -p "$OUT"

echo "=== Building Linux (amd64) ==="
docker build -f Dockerfile.build -t svema-build .
docker create --name tmp-svema svema-build
docker cp tmp-svema:/svema_uploader_linux "$OUT/svema_uploader_linux"
docker rm tmp-svema
echo "Done: $OUT/svema_uploader_linux"

echo ""
echo "Built binaries:"
ls -lh "$OUT/"
