@echo off
echo Building nwafu-srun for Windows...
set GOOS=windows
set GOARCH=amd64
go build -o nwafu-srun.exe main.go
echo Done: nwafu-srun.exe
