# ProIdentity

A cross-platform VPN client built on the WireGuard® protocol, with a desktop GUI and a system daemon. Supports both standalone tunnel management and managed-mode operation against a central management server.

**Platform support:** Windows (primary), macOS

> WireGuard is a registered trademark of Jason A. Donenfeld.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

---

## Features

- Import and manage WireGuard tunnels from `.conf` files
- Connect/disconnect tunnels with live traffic statistics (RX/TX, last handshake)
- Daemon runs as a system service (auto-starts on boot), GUI connects to it on launch
- **Standalone mode** — tunnels stored locally, encrypted with AES-256-GCM
- **Managed mode** — authenticate against a management server, connect to server-provisioned VPN sessions, sync user-uploaded configs; all traffic encrypted end-to-end between client and server (X25519 ECDH + AES-256-GCM)
- Setup wizard for first-run configuration
- TOTP (2FA) support in managed mode

---

## License

ProIdentity Access is source-available under the PolyForm Noncommercial
License 1.0.0 for noncommercial use. Commercial, enterprise, MSP, resale,
hosted-service, or other revenue-generating use requires a separate written
commercial license from Pro-IT-Services. See [LICENSE](LICENSE).

---

## Architecture

```
┌─────────────────────────────────────┐
│        GUI (Wails / React)          │
│  React + TypeScript + Zustand       │
│  Dark themed, 960x660 window        │
└──────────────┬──────────────────────┘
               │ JSON-RPC 2.0
               │ Windows named pipe  \\.\pipe\proidentity
               │ Unix socket         /var/run/proidentity.sock
               │ Token auth (per-session file-based token)
┌──────────────▼──────────────────────┐
│     Daemon (ProIdentity Daemon.exe) │
│  Windows Service / macOS LaunchD    │
│  Manages tunnels, routes, DNS       │
│  Persists configs (AES-256-GCM)     │
└──────────────┬──────────────────────┘
               │ WireGuard userspace (golang.zx2c4.com/wireguard)
               │ Wintun TUN driver (Windows, embedded in binary)
┌──────────────▼──────────────────────┐
│       OS Networking Stack           │
│  TUN device, routes, DNS            │
└─────────────────────────────────────┘
```

The GUI and daemon are **separate binaries** that communicate over a local IPC channel. The daemon runs as `LocalSystem` (Windows) or `root` (macOS) and handles all privileged WireGuard operations. The GUI runs as a normal user process.

---

## Installation

### Windows — MSI Installer

Download `ProIdentity-<version>.msi` and run it. The installer:

- Installs `ProIdentity.exe` and `ProIdentity Daemon.exe` to `C:\Program Files\ProIdentity\`
- Registers `ProIdentity` as a Windows Service (auto-start, LocalSystem)
- Starts the service immediately
- Creates a Start Menu shortcut for the GUI

**Uninstalling** via Add/Remove Programs stops and removes the service automatically.

### macOS

Build from source (see below) and run the daemon installer:

```bash
sudo ./daemon -install
```

This installs a LaunchDaemon plist at `/Library/LaunchDaemons/com.proidentity.app.plist` (auto-start, kept alive). Then launch `ProIdentity.app`.

---

## Building from Source

### Prerequisites

| Tool | Install |
|------|---------|
| Go 1.22+ | https://go.dev/dl |
| Wails v2 CLI | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| Node.js 18+ | https://nodejs.org |

### Full build (UI + App + Daemon + MSI)

```bat
build.bat
```

Or from PowerShell:

```powershell
.\build.ps1
.\build.ps1 -Version 0.2.0   # override version
.\build.ps1 -SkipUI           # skip npm steps (reuse existing frontend/dist)
```

Requires WiX v4 for the MSI step:

```powershell
dotnet tool install --global wix --version "4.*"
wix extension add WixToolset.UI.wixext --global
```

### Individual steps

```bash
# GUI
wails build -platform windows/amd64

# Daemon
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" \
  -o "build/bin/ProIdentity Daemon.exe" ./cmd/daemon
```

Output: `build/bin/ProIdentity.exe`, `build/bin/ProIdentity Daemon.exe`, `build/ProIdentity-0.1.0.msi`

---

## Daemon

The daemon (`ProIdentity Daemon.exe` on Windows, `daemon` on macOS) handles all WireGuard tunnel operations. It runs as a privileged system service and exposes a local IPC socket.

### Service Management (Windows)

```powershell
# Install service (done automatically by MSI)
.\daemon.exe -install

# Uninstall service
.\daemon.exe -uninstall

# Check service status
sc query ProIdentity

# Start / stop manually
Start-Service ProIdentity
Stop-Service ProIdentity
```

### Config Storage

| Platform | Location |
|----------|----------|
| Windows | `C:\ProgramData\ProIdentity\tunnels\` |
| macOS | `/Library/Application Support/ProIdentity/tunnels/` |

Each tunnel is stored as `<uuid>.json`. In standalone mode with encryption enabled, files are AES-256-GCM encrypted (the GUI sets the encryption key over IPC on startup).

---

## IPC Protocol

The GUI communicates with the daemon via **JSON-RPC 2.0** over a local socket or named pipe.

### Authentication

On startup the daemon generates a random 32-byte token and writes it to:

| Platform | Token file |
|----------|------------|
| Windows | `C:\ProgramData\ProIdentity\ipc.token` |
| Unix | `/var/run/proidentity.token` |

The GUI reads this file and sends `AUTH <hex-token>\n` on connect. The daemon replies `OK\n` or `DENIED\n`. All subsequent messages are JSON-RPC.

### RPC Methods

| Method | Description |
|--------|-------------|
| `tunnel.list` | List all imported tunnels |
| `tunnel.import` | Import and persist a tunnel from `.conf` content |
| `tunnel.import_ephemeral` | Import a tunnel without writing to disk |
| `tunnel.delete` | Delete a tunnel |
| `tunnel.connect` | Bring up a tunnel |
| `tunnel.disconnect` | Tear down a tunnel |
| `tunnel.stats` | Get RX/TX bytes and last handshake timestamp |
| `tunnel.export` | Export tunnel config as `.conf` text |
| `daemon.status` | Get daemon version |
| `daemon.set_encryption_key` | Set the AES-256 key for on-disk config encryption |

### Server-Pushed Events

| Event | Payload | Description |
|-------|---------|-------------|
| `tunnel.changed` | `TunnelInfo` | Tunnel state changed (connecting/connected/error/disconnected) |
| `stats.update` | `StatsInfo` | Traffic stats, pushed every 2 seconds while tunnel is up |

---

## Modes of Operation

### Standalone Mode

- Import tunnels from standard WireGuard `.conf` files
- Configs stored locally under `C:\ProgramData\ProIdentity\tunnels\`
- On-disk encryption with AES-256-GCM (key stored per-user in app data)
- Works fully offline

### Managed Mode

Connect to a central management server for organisation-wide VPN management.

**Setup wizard steps:**

1. Choose mode (standalone / managed)
2. Enter management server URL
3. Register device — generates an X25519 keypair; the server returns its public key; a shared AES-256 key is derived via ECDH + HKDF-SHA256 and used to encrypt all subsequent API traffic
4. Log in with username + password (+ optional TOTP code)
5. Setup complete

**Once logged in:**

- Browse available VPN servers
- Connect to a server: the server provisions a WireGuard session (assigns IP, sends config), the client injects a session keypair and connects via the daemon
- Upload personal `.conf` files to the server (encrypted with a per-user server key)
- Configs sync automatically every 30 seconds

**Session keepalive:** Each active server connection sends a keepalive to the management server every 30 seconds to maintain the session.

**Device revocation:** If the server revokes a device, the client detects the `device revoked` error on the next API call, disconnects all tunnels, wipes all local credentials, and resets to the setup wizard.

---

## Security

### IPC

- Authentication via a per-session random token written to a local file
- Only local processes that can read the token file can connect
- Named pipe / Unix socket — no network exposure

### On-Disk Encryption (Standalone)

- Algorithm: AES-256-GCM
- Key: 32 bytes, stored in user app data, set on the daemon over IPC at startup
- Format: `nonce (12 B) || ciphertext || tag (16 B)`
- Auto-migration: plaintext configs from older versions are re-encrypted when the key is loaded

### Managed API Transport

- Device registration derives a shared AES-256 key via X25519 ECDH + HKDF-SHA256
- Every API request/response body is encrypted with this key (AES-256-GCM)
- The Device ID is used as Additional Authenticated Data (AAD) — request cannot be replayed against a different device
- Bearer token (JWT) added to all authenticated requests

### User Configs (Managed)

- Configs uploaded to the server are encrypted client-side with a per-user key fetched from the server
- Configs are never written to local disk in managed mode

### Privilege Separation

| Component | Privilege |
|-----------|-----------|
| Daemon | LocalSystem (Windows) / root (macOS) |
| GUI | Normal user |
| IPC channel | Local-only, token-authenticated |

---

## Project Structure

```
.
├── main.go                        Wails entry point
├── app.go                         GUI backend — all methods exposed to frontend
├── cmd/daemon/main.go             Daemon entry point (-install / -uninstall / run)
├── internal/
│   ├── config/                    WireGuard .conf parser and data model
│   ├── crypto/                    AES-256-GCM utilities
│   ├── daemon/
│   │   ├── tunnel.go              Single tunnel lifecycle (TUN device, routes, DNS)
│   │   ├── tunnel_manager.go      Manages all tunnels, persistence, stats loop
│   │   ├── routes_windows.go      Windows route management (netsh / route)
│   │   ├── routes_darwin.go       macOS route management
│   │   ├── wintun_windows.go      Wintun DLL extraction (embedded in binary)
│   │   └── platform/
│   │       ├── service_windows.go Windows Service install/uninstall/handler
│   │       └── service_darwin.go  macOS LaunchDaemon install/uninstall/handler
│   ├── ipc/
│   │   ├── types.go               JSON-RPC request/response/event types
│   │   ├── server.go              IPC server (daemon side)
│   │   ├── client.go              IPC client (GUI side)
│   │   ├── auth.go                Token generation and validation
│   │   ├── socket_windows.go      Windows named pipe
│   │   └── socket_unix.go         Unix socket
│   └── managed/
│       ├── client.go              Management server HTTP client
│       ├── devcrypto.go           X25519 keypair + ECDH key derivation
│       └── settings.go            Persistent managed-mode settings
├── frontend/
│   └── src/
│       ├── App.tsx                Root component and event routing
│       ├── components/            UI components (tunnel cards, modals, panels)
│       ├── stores/                Zustand state stores
│       ├── wailsbridge.ts         Wails runtime wrapper
│       └── types/                 TypeScript type definitions
├── installer/
│   ├── Product.wxs                WiX v4 MSI installer definition
│   ├── License.rtf                License shown in the installer dialog
│   └── build.ps1                  Forwards to root build.ps1
├── build.ps1                      Full build script (UI + App + Daemon + MSI)
├── build.bat                      build.ps1 wrapper (bypasses execution policy)
└── build/
    └── bin/                       Compiled binaries
```

---

## Development

Run in development mode (hot-reload frontend):

```bash
wails dev
```

The daemon must already be running (either as a service or started manually):

```bash
go run ./cmd/daemon
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/wailsapp/wails/v2` | Desktop GUI framework |
| `golang.zx2c4.com/wireguard` | WireGuard® userspace implementation |
| `golang.zx2c4.com/wintun` | Windows TUN driver |
| `github.com/Microsoft/go-winio` | Windows named pipes |
| `golang.org/x/crypto` | X25519, HKDF |
| `golang.org/x/sys` | Windows Service control, syscalls |
| `github.com/google/uuid` | Tunnel UUID generation |
