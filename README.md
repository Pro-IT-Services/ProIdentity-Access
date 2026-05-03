# ProIdentity Access

ProIdentity Access is a managed VPN access platform built around WireGuard.
It includes a Go management server, a desktop client with a privileged local
daemon, and mobile clients for Android and iOS.

WireGuard is a registered trademark of Jason A. Donenfeld.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

## What Is Included

- `server/` - Go API server, admin Web UI, WireGuard session management, MariaDB migrations, and Docker deployment files.
- `client/` - Windows and macOS desktop client built with Wails, React, and Go.
- `android/` - Android client using Kotlin, Compose, and the WireGuard Android tunnel library.
- `ios/` - iOS client and Packet Tunnel extension.

## Recommended Server Deployment

For production, use the server release archive from GitHub Releases. It
contains the compiled server binary, database migrations, an example config,
and a systemd unit.

Requirements:

- Linux `amd64`
- MariaDB 10.6 or newer
- `curl`, `tar`, `sha256sum`, and `systemd`
- WireGuard tooling, TUN support, and firewall tooling on the host

Create the database first:

```sql
CREATE DATABASE proidentity CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'proidentity'@'localhost' IDENTIFIED BY 'change-this-db-password';
GRANT ALL PRIVILEGES ON proidentity.* TO 'proidentity'@'localhost';
FLUSH PRIVILEGES;
```

Install from a release:

```sh
curl -fsSL -o install-release.sh \
  https://raw.githubusercontent.com/Pro-IT-Services/ProIdentity-Access/main/server/install-release.sh
sudo sh install-release.sh Pro-IT-Services/ProIdentity-Access 0.5.19
```

The installer downloads and verifies:

```text
ProIdentity-Access-Server-0.5.19-linux-amd64.tar.gz
ProIdentity-Access-0.5.19-SHA256SUMS.txt
```

It installs the server under `/opt/proidentity`, writes config files under
`/etc/proidentity`, installs `proidentity.service`, and prints the initial admin
password if it creates one.

Before starting, edit the database DSN:

```sh
sudo nano /etc/proidentity/config.yaml
```

Start the service:

```sh
sudo systemctl start proidentity
sudo systemctl status proidentity --no-pager
```

The HTTP service binds to `127.0.0.1:8080` by default. Put a TLS reverse proxy
in front of it and expose only the WireGuard UDP ports you need.

Upgrade from a newer release by running the installer again with the new
version, then restart the service:

```sh
sudo sh install-release.sh Pro-IT-Services/ProIdentity-Access 0.5.19
sudo systemctl restart proidentity
```

## Docker Server Deployment

Use the release-binary deployment above for the simplest production setup.

The Docker deployment is intended for operators who want the server, MariaDB,
and WireGuard runtime managed by Docker Compose from a checked-out copy of this
repository. It builds the server container locally from `server/Dockerfile`,
then runs it inside a normal Docker bridge network. It does not require host
networking, but the Docker host must provide `/dev/net/tun`, `NET_ADMIN`, and
UDP port forwarding for WireGuard.

On a Linux server with Docker and Docker Compose:

```sh
git clone https://github.com/Pro-IT-Services/ProIdentity-Access.git
cd ProIdentity-Access
cd server/docker
sudo ./host-prep.sh
./up.sh
```

On first run, `up.sh` creates `server/docker/.env` with random database, JWT,
and admin secrets. It prints the initial admin password once.

Useful commands:

```sh
cd server/docker
./logs.sh
./backup.sh
./down.sh
```

Default published ports:

- Web UI/API: `127.0.0.1:8080`
- WireGuard UDP: `51820-51840/udp`

If you need a different HTTP bind address, port, or UDP range, edit
`server/docker/.env` before running `./up.sh`:

```env
PROIDENTITY_HTTP_BIND=127.0.0.1
PROIDENTITY_HTTP_PORT=8080
PROIDENTITY_WG_UDP_PORTS=51820-51840
PROIDENTITY_TRUSTED_PROXIES=127.0.0.1,172.16.0.0/12,10.0.0.0/8,192.168.0.0/16
```

Keep managed WireGuard servers inside the published UDP range unless you also
update the Compose port mapping.

## Reverse Proxy

Put your company reverse proxy in front of the HTTP service:

```text
https://vpn.example.com -> http://127.0.0.1:8080
```

Nginx example:

```nginx
server {
    listen 443 ssl http2;
    server_name vpn.example.com;

    ssl_certificate /etc/letsencrypt/live/vpn.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/vpn.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

Also allow or forward the selected WireGuard UDP ports from the public network
to the Docker host.

## First Server Setup

1. Open the Web UI through your reverse proxy or `http://127.0.0.1:8080`.
2. Sign in with the initial admin account from `.env`.
3. Change the admin password.
4. Add VPN servers.
5. Add resources and bundles.
6. Assign access by user:

```text
USER <> SERVERS <> BUNDLES <> RESOURCES
```

Users can then sign in from the desktop or mobile clients, register their
device, and connect to the VPNs they are allowed to use.

## Manual Server Build

Requirements:

- Go 1.24 or newer
- Node.js 22 or newer
- MariaDB
- Linux with WireGuard tools, `iproute2`, firewall tooling, and TUN support

Build the embedded Web UI and server binary:

```sh
cd server
cd webui
npm ci
npm run build
cd ..
rm -rf internal/api/ui/dist
cp -r webui/dist internal/api/ui/dist
go build -trimpath -o bin/proidentity ./cmd/server
```

Run locally with a configured database:

```sh
export WG_ADMIN_USER=admin
export WG_ADMIN_PASS='change-this-password'
export WG_ADMIN_EMAIL=admin@example.com
./bin/proidentity config.yaml
```

Database migrations are applied automatically when the server starts.

## Desktop Client Usage

Download the desktop installer from
https://github.com/Pro-IT-Services/ProIdentity-Access/releases.

Release assets use these names:

```text
ProIdentity-Access-0.5.19.msi
ProIdentity-Access-0.5.19.pkg
```

Install the Windows MSI or macOS PKG, launch ProIdentity Access, and follow the
setup wizard.

Managed mode:

1. Enter the server URL, for example `https://vpn.example.com`.
2. Register the device.
3. Sign in with a server user account.
4. Connect to an assigned VPN.

Standalone mode:

1. Import a standard WireGuard `.conf` file.
2. Connect or disconnect from the desktop app or tray window.

## Build Desktop Clients From Source

Common requirements:

- Go 1.22 or newer
- Node.js 18 or newer
- Wails v2:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Windows MSI

Additional requirements:

- .NET SDK
- WiX v4

```powershell
dotnet tool install --global wix --version "4.*"
wix extension add WixToolset.UI.wixext/4.0.6 --global
wix extension add WixToolset.Util.wixext/4.0.6 --global
cd client
powershell -ExecutionPolicy Bypass -File .\build.ps1 -Version 0.5.19
```

Output:

```text
client/build/ProIdentity-Access-0.5.19.msi
```

### macOS PKG

Additional requirements:

- macOS
- Xcode Command Line Tools
- `pkgbuild`

```sh
cd client
./build.sh --version 0.5.19
```

Output:

```text
client/build/darwin/ProIdentity-Access-0.5.19.pkg
```

Install locally:

```sh
sudo installer -pkg client/build/darwin/ProIdentity-Access-0.5.19.pkg -target /
```

## Build Android From Source

Requirements:

- Android Studio or Android SDK command line tools
- JDK 17
- Gradle
- Node.js and npm for the embedded frontend assets

Build frontend assets and an APK:

```sh
cd android
./build-frontend.sh
gradle assembleDebug
```

For a release build, configure signing in Android Studio or your Gradle
environment, then run:

```sh
gradle assembleRelease
```

The app version is configured in `android/app/build.gradle.kts`.

## Build iOS From Source

Requirements:

- macOS
- Xcode
- Apple Developer account with Network Extension capability

Open the project:

```sh
open ios/ProIdentity.xcodeproj
```

In Xcode:

1. Select your development team.
2. Confirm bundle identifiers for the app and Packet Tunnel extension.
3. Confirm Network Extension entitlements.
4. Build and run the `ProIdentity` target on a physical device.

The iOS simulator cannot establish a real Packet Tunnel VPN session.

## Security Notes For Production

- Use HTTPS for every client-facing deployment.
- Keep `/etc/proidentity/proidentity.env`, `/etc/proidentity/config.yaml`, and
  `server/docker/.env` private and backed up securely.
- Rotate the initial admin password immediately.
- Restrict admin accounts.
- Keep database and Docker volumes backed up.
- Publish only the WireGuard UDP ports you actually use.
- Do not expose the server's HTTP port directly to the internet without a
  reverse proxy and TLS.
- Verify `/dev/net/tun` support before deploying inside LXC or other restricted
  container environments.

## Tests

Go tests:

```sh
go test ./...
```

Server Web UI build:

```sh
cd server/webui
npm ci
npm run build
```

Client frontend build:

```sh
cd client/frontend
npm ci
npm run build
```

## License

ProIdentity Access is source-available under the PolyForm Noncommercial
License 1.0.0 for noncommercial use.

Commercial, enterprise, MSP, resale, hosted-service, or other
revenue-generating use requires a separate written commercial license from
Pro-IT-Services. See `LICENSE` before using, modifying, redistributing, or
deploying this project.
