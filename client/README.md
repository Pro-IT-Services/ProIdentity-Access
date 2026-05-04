# ProIdentity Access Desktop Client

ProIdentity Access Desktop is a Windows and macOS VPN client built on
WireGuard. It includes a user-facing app and a privileged local daemon so
normal users can connect and disconnect VPN sessions without running the app as
administrator.

WireGuard is a registered trademark of Jason A. Donenfeld.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

## Install From Releases

Download installers from:

https://github.com/Pro-IT-Services/ProIdentity-Access/releases

Current release asset names:

```text
ProIdentity-Access-0.5.24.msi
ProIdentity-Access-0.5.24.pkg
ProIdentity-Access-0.5.24-SHA256SUMS.txt
```

### Windows MSI

Run `ProIdentity-Access-0.5.24.msi`.

The installer:

- Installs the desktop app and daemon under `C:\Program Files\ProIdentity\`.
- Registers the daemon as the `ProIdentity` Windows service.
- Runs the daemon as `LocalSystem`.
- Starts the service immediately.
- Adds a Start Menu shortcut for ProIdentity Access.

Users do not need to run the desktop app as administrator. The daemon performs
privileged tunnel operations.

### macOS PKG

Run `ProIdentity-Access-0.5.24.pkg`.

The package installs:

- `ProIdentity Access.app`
- A root LaunchDaemon for privileged tunnel operations
- Supporting uninstall scripts

After installation, launch ProIdentity Access and follow the setup wizard.

## Managed Mode Setup

1. Open ProIdentity Access.
2. Choose managed setup.
3. Enter the server URL, for example `https://vpn.example.com`.
4. Register the device.
5. Sign in with your VPN account.
6. Connect to an assigned VPN.

Managed mode receives VPN sessions from the ProIdentity Access server. If the
server expires or revokes the session, the client clears local managed state and
returns to the setup wizard.

## Standalone Mode

Standalone mode lets users import standard WireGuard `.conf` files. Local
tunnel data is stored by the daemon and encrypted at rest where supported.

## Build From Source

Common requirements:

- Go 1.22 or newer
- Node.js 18 or newer
- Wails v2

Install Wails:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Windows MSI Build

Additional requirements:

- .NET SDK
- WiX v4

```powershell
dotnet tool install --global wix --version "4.*"
wix extension add WixToolset.UI.wixext/4.0.6 --global
wix extension add WixToolset.Util.wixext/4.0.6 --global
cd client
powershell -ExecutionPolicy Bypass -File .\build.ps1 -Version 0.5.24
```

Output:

```text
client/build/ProIdentity-Access-0.5.24.msi
```

### macOS PKG Build

Additional requirements:

- macOS
- Xcode Command Line Tools
- `pkgbuild`

```sh
cd client
./build.sh --version 0.5.24
```

Output:

```text
client/build/darwin/ProIdentity-Access-0.5.24.pkg
```

Install locally:

```sh
sudo installer -pkg client/build/darwin/ProIdentity-Access-0.5.24.pkg -target /
```

## Architecture

The desktop client is split into two processes:

```text
Desktop app (Wails / React)
  -> local IPC socket or Windows named pipe
Privileged daemon (LocalSystem on Windows, root on macOS)
  -> WireGuard userspace, TUN device, routes, DNS
Operating system networking stack
```

The app handles user interaction. The daemon owns privileged networking,
tunnel lifecycle, route updates, DNS updates, and local tunnel storage.

## Service Management

Windows service commands:

```powershell
sc query ProIdentity
Start-Service ProIdentity
Stop-Service ProIdentity
```

macOS LaunchDaemon commands:

```sh
sudo launchctl list | grep -i proidentity
sudo launchctl kickstart -k system/com.proitservices.proidentity.access.daemon
```

## Security Notes

- The daemon IPC endpoint is local-only and token authenticated.
- The desktop app runs as a normal user.
- The daemon performs privileged WireGuard operations.
- Managed API traffic is encrypted during device registration and authenticated
  requests.
- Managed session expiration, device revocation, and auth failures reset the app
  to the setup wizard.
- Do not commit local configs, logs, build outputs, signing keys, or installer
  artifacts.

## License

ProIdentity Access is free for personal, internal, and company use under the
ProIdentity Access Free Internal Use License 1.0. Redistribution, resale,
hosted-service/MSP/provider use, white-labeling, and sharing modified builds
require prior written permission from Pro-IT-Services. See the repository root
`LICENSE`.
