@echo off
echo Building nwafu-srun for Windows...
set GOOS=windows
set GOARCH=amd64
go build -o nwafu-srun.exe main.go
echo Done.

echo Building nwafu-srun for Linux...
set GOOS=linux
set GOARCH=amd64
go build -o nwafu-srun-linux main.go
echo Done.

echo Building nwafu-srun for macOS (Intel)...
set GOOS=darwin
set GOARCH=amd64
go build -o nwafu-srun-darwin main.go
echo Done.

echo Building nwafu-srun for macOS (Apple Silicon)...
set GOOS=darwin
set GOARCH=arm64
go build -o nwafu-srun-darwin-arm64 main.go
echo Done.
