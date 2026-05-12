# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

A Go CLI tool for Srun (深澜) campus network authentication at Northwest A&F University. It logs in, logs out, and queries account status via the Srun portal API.

## Build & Run

```bash
go build -o nwafu-srun main.go     # local platform
./build.sh                          # cross-compile linux/darwin/windows
./build.bat                         # same, for Windows
```

No external dependencies — the Go module uses only standard library packages.

## Architecture

```
main.go                 # CLI: flag parsing (-u/-p/-f/-v), interactive menu loop
pkg/srun/
  client.go             # HTTP client for the Srun portal API (login/logout/status)
  crypto.go             # Custom Srun encryption: XTEA variant + custom Base64 + HMAC-MD5 + SHA1
```

**Authentication flow** (4 steps, all in `client.go`):
1. `GetIP()` — fetches the login page, extracts the client IP from embedded JS
2. `GetChallenge()` — gets an auth challenge token from `/cgi-bin/get_challenge`
3. `LogIn()` — sends encrypted credentials to `/cgi-bin/srun_portal`, then calls `GetLoginInfo()` on success
4. `GetLoginInfo()` — queries `/cgi-bin/rad_user_info` for account status/balance/usage

**Fallback behavior**: `probeAndSetBaseURL()` tests DNS resolution of `portal.nwafu.edu.cn` and falls back to IP `172.26.8.11` (HTTP, TLS verification disabled) when DNS is unreachable due to being offline.

**Cookie persistence**: Uses `net/http/cookiejar` so `JSESSIONID` from the initial GET is automatically included in subsequent requests, matching browser behavior.

**Two operational modes**:
- Interactive (default): menu loop with login/logout/status/exit choices
- Force mode (`-f`): logout then login without prompting, suitable for cron/scripts

**Verbose mode (`-v`)**: prints request URLs and response bodies (truncated at 200 chars for the login page) for debugging.

## Known issues

- Logout may not work due to upstream Srun system issues
- Account info may not be available immediately after login
