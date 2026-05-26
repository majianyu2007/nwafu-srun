# CLAUDE.md

Guidance for working on this repository.

## Overview

Go CLI for Srun (深澜) campus network auth at NWAFU: login, logout, status, bypass billing, and optional **config-file auto-auth**.

## Build

```bash
go build -o nwafu-srun .
go build -o utils/bypass/bypass ./utils/bypass   # local only; not in CI release artifacts
go test ./...
```

Deps: `golang.org/x/term`.

## Architecture

```
main.go                    # CLI, interactive menu, config merge, auto-auth
utils/bypass/main.go       # Standalone bypass (reads config username)
pkg/config/                # JSON config in user config dir only
pkg/srun/
  const.go, logger.go, http_util.go
  client.go                # Portal API
  crypto.go
  selfservice.go           # SSO, sessions, RunBypass
  errors.go                # Sentinel errors + Hint()
```

## Configuration

- Config path: `os.UserConfigDir()/nwafu-srun/config.json` (override with `--config`)
- Merge priority: **CLI > env > file**
- Non-interactive when: explicit `-u/-p`, `auto_auth` in config, or `-f`/`-b` with saved creds
- Passwords stored **plaintext**, mode `0600`, Windows hidden attribute on config file
- Flags: `--no-config`, `--save-config`

## Interactive menu

1 Login (online check) · 2 Force re-login · 3 Logout · 4 Status · 5 Bypass · 6 Settings · 7 Exit

After successful login: optional save prompt (y / n / Never).

## Errors

Use `errors.Is` with sentinels in `errors.go`. Print `srun.Hint(err)` for user remediation.

## CLI (main)

| Flag | Description |
|------|-------------|
| `-u`, `-p` | Credentials |
| `-f` | Pre-login logout |
| `-b` | Post-login bypass |
| `-a` | Kick all devices on bypass |
| `--config`, `--no-config`, `--save-config` | Config file control |

Env: `NWAFU_SRUN_USERNAME`, `NWAFU_SRUN_PASSWORD`.

## Bypass tool

`utils/bypass`: `-u`, `--login`, `-a`, reads config if `-u` omitted.

## Exit codes

0 OK · 1 runtime · 2 usage/config
