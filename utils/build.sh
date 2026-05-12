#!/bin/bash
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)       echo "Unknown architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    darwin)  BINARY="nwafu-srun-darwin-${ARCH}" ;;
    linux)   BINARY="nwafu-srun-linux-${ARCH}" ;;
    *)       echo "Unknown OS: $OS"; exit 1 ;;
esac

echo "Building nwafu-srun for ${OS}/${ARCH}..."
go build -o "$BINARY" main.go
echo "Done: $BINARY"
