#!/bin/bash
echo "Building nwafu-srun for Linux..."
GOOS=linux GOARCH=amd64 go build -o nwafu-srun-linux main.go
echo "Done."

echo "Building nwafu-srun for macOS (Intel)..."
GOOS=darwin GOARCH=amd64 go build -o nwafu-srun-darwin main.go
echo "Done."

echo "Building nwafu-srun for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -o nwafu-srun-darwin-arm64 main.go
echo "Done."

echo "Building nwafu-srun for Windows..."
GOOS=windows GOARCH=amd64 go build -o nwafu-srun.exe main.go
echo "Done."
