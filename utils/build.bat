@echo off
echo Building nwafu-srun for Windows...
set GOOS=windows
set GOARCH=amd64
go build -o nwafu-srun.exe .
go build -o utils\bypass\bypass.exe .\utils\bypass
echo Done: nwafu-srun.exe, utils\bypass\bypass.exe
