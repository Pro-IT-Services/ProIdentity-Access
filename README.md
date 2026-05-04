# ProIdentity Access

ProIdentity Access is a managed VPN access platform built around WireGuard. It
includes a Go management server, an admin Web UI, a Windows/macOS desktop
client with a privileged local daemon, and Android/iOS clients.

WireGuard is a registered trademark of Jason A. Donenfeld.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

## What Is Included

- `server/` - Go API server, admin Web UI, WireGuard session management,
  MariaDB migrations, systemd service, release installer, and Docker deployment
  files.
- `client/` - Windows and macOS desktop client built with Wails, React, and Go.
- `android/` - Android client using Kotlin, Compose, and the WireGuard Android
  tunnel library.
- `ios/` - iOS client and Packet Tunnel extension.

## Recommended Production Setup

Recommended server OS:

- Debian 13 "Trixie" `amd64`

Recommended deployment method:

- Server: GitHub Release archive installed by `server/install-release.sh`
- Database: MariaDB on the same host or a private database host
- HTTP exposure: Nginx, Caddy, Traefik, or another TLS reverse proxy
- VPN traffic: selected WireGuard UDP ports forwarded to the server
- Clients: Windows MSI, macOS PKG, Android APK/AAB, or iOS build installed
  separately

Use Docker only when your company wants Compose-managed services. The Docker
deployment builds the server image locally from a checked-out source tree; the
release-binary systemd deployment is the recommended default.

## Network Requirements

Plan these values before installation:

| Item | Example | Notes |
| --- | --- | --- |
| Public URL | `https://vpn.example.com` | Used by admins and clients |
| HTTP backend | `127.0.0.1:8080` | ProIdentity listens locally by default |
| TLS ports | `80/tcp`, `443/tcp` | Used by the reverse proxy |
| WireGuard UDP range | `51820-51840/udp` | Publish only the ports you assign to VPN servers |
| Database | `proidentity` | MariaDB database name |
| Database user | `proidentity` | Use a strong unique password |

Do not expose the raw ProIdentity HTTP port directly to the internet. Put TLS
in front of it.

## Debian 13 Server Packages

Start from a fresh Debian 13 server and run as `root` or with `sudo`.

```sh
sudo apt update
sudo apt install -y \
  ca-certificates \
  curl \
  tar \
  gzip \
  openssl \
  coreutils \
  git \
  mariadb-server \
  mariadb-client \
  wireguard-tools \
  iproute2 \
  iptables \
  nftables \
  procps \
  nginx \
  certbot \
  python3-certbot-nginx
```

Enable required services:

```sh
sudo systemctl enable --now mariadb
sudo systemctl enable --now nginx
```

Enable IPv4 forwarding for WireGuard:

```sh
cat <<'EOF' | sudo tee /etc/sysctl.d/99-proidentity.conf
net.ipv4.ip_forward=1
net.ipv4.conf.all.src_valid_mark=1
EOF

sudo sysctl --system
```

Verify TUN support:

```sh
test -c /dev/net/tun && echo "TUN device is available"
```

If this fails inside LXC, VPS control panels, or other restricted environments,
enable TUN support on the host before continuing.

## Database Setup

Create a database, a dedicated database user, and a strong password:

```sh
sudo mariadb
```

```sql
CREATE DATABASE proidentity CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'proidentity'@'localhost' IDENTIFIED BY 'change-this-db-password';
GRANT ALL PRIVILEGES ON proidentity.* TO 'proidentity'@'localhost';
FLUSH PRIVILEGES;
EXIT;
```

If MariaDB runs on another private host, replace `localhost` with the server IP
or hostname that will connect to the database, then use that host in the DSN.

Recommended local DSN:

```text
proidentity:change-this-db-password@tcp(127.0.0.1:3306)/proidentity
```

## Install Server From GitHub Release

Download the installer script from the public repository:

```sh
curl -fsSL -o install-release.sh \
  https://raw.githubusercontent.com/Pro-IT-Services/ProIdentity-Access/main/server/install-release.sh
sudo sh install-release.sh Pro-IT-Services/ProIdentity-Access 0.5.24
```

The installer downloads and verifies these release assets:

```text
ProIdentity-Access-Server-0.5.24-linux-amd64.tar.gz
ProIdentity-Access-0.5.24-SHA256SUMS.txt
```

It installs:

| Path | Purpose |
| --- | --- |
| `/opt/proidentity/bin/proidentity` | Server binary |
| `/opt/proidentity/migrations/` | Database migrations |
| `/etc/proidentity/config.yaml` | Main server config |
| `/etc/proidentity/proidentity.env` | Secret environment values |
| `/opt/proidentity/VERSION` | Installed server release version |
| `/opt/proidentity/update-release.sh` | Release updater script |
| `/etc/systemd/system/proidentity.service` | systemd service |

If `/etc/proidentity/proidentity.env` did not exist, the installer creates it
and prints the generated initial admin password and database credentials once.
If `PROIDENTITY_DATABASE_DSN` is not provided, it generates a database user,
database password, and DSN in `proidentity.env`. It also prints SQL that can be
run on MariaDB if the database and user do not already exist.

## Configure Server

Edit the main config:

```sh
sudo nano /etc/proidentity/config.yaml
```

Minimal production example:

```yaml
server:
  host: "127.0.0.1"
  port: 8080
  cors_origins:
    - "https://vpn.example.com"

database:
  dsn: "proidentity:change-this-db-password@tcp(127.0.0.1:3306)/proidentity"

auth:
  jwt_secret: "set-with-PROIDENTITY_JWT_SECRET"
```

Config fields:

| Field | Required | Description |
| --- | --- | --- |
| `server.host` | Yes | Bind address. Use `127.0.0.1` behind a reverse proxy. |
| `server.port` | Yes | HTTP port used by the reverse proxy. Default example is `8080`. |
| `server.cors_origins` | Yes | Allowed browser origins. Include your public HTTPS URL. |
| `database.dsn` | Yes | MariaDB DSN in `user:pass@tcp(host:port)/database` format. |
| `auth.jwt_secret` | Yes | JWT signing secret. `PROIDENTITY_JWT_SECRET` overrides this value when set. |

Check secret env values:

```sh
sudo nano /etc/proidentity/proidentity.env
```

Environment fields:

| Variable | Required | Description |
| --- | --- | --- |
| `PROIDENTITY_JWT_SECRET` | Yes | Long random JWT signing secret. Keep private. |
| `PROIDENTITY_DATABASE_DSN` | Optional | Overrides `database.dsn` from `config.yaml`. |
| `PROIDENTITY_DATABASE_NAME` | Optional install input | Used by `install-release.sh` when generating a DSN. |
| `PROIDENTITY_DATABASE_USER` | Optional install input | Used by `install-release.sh` when generating a DSN. |
| `PROIDENTITY_DATABASE_PASS` | Optional install input | Used by `install-release.sh` when generating a DSN. |
| `PROIDENTITY_DATABASE_HOST` | Optional install input | Defaults to `127.0.0.1:3306` when generating a DSN. |
| `PROIDENTITY_SERVER_HOST` | Optional | Overrides `server.host` from `config.yaml`. |
| `PROIDENTITY_SERVER_PORT` | Optional | Overrides `server.port` from `config.yaml`. |
| `WG_ADMIN_USER` | Yes on first boot | Initial admin username. |
| `WG_ADMIN_PASS` | Yes on first boot | Initial admin password. Rotate after first login. |
| `WG_ADMIN_EMAIL` | Recommended | Initial admin email. |

Generate a replacement JWT secret if needed:

```sh
openssl rand -base64 48
```

Lock down local config files:

```sh
sudo chown -R root:root /etc/proidentity
sudo chmod 0750 /etc/proidentity
sudo chmod 0640 /etc/proidentity/config.yaml
sudo chmod 0600 /etc/proidentity/proidentity.env
```

## Start Server

```sh
sudo systemctl daemon-reload
sudo systemctl start proidentity
sudo systemctl status proidentity --no-pager
```

Follow logs:

```sh
sudo journalctl -u proidentity -f
```

The server applies database migrations automatically on startup.

Check the local HTTP endpoint:

```sh
curl -i http://127.0.0.1:8080/
```

## Reverse Proxy With Nginx

Create a site config:

```sh
sudo nano /etc/nginx/sites-available/proidentity
```

Example:

```nginx
server {
    listen 80;
    server_name vpn.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable the site and request TLS:

```sh
sudo ln -s /etc/nginx/sites-available/proidentity /etc/nginx/sites-enabled/proidentity
sudo nginx -t
sudo systemctl reload nginx
sudo certbot --nginx -d vpn.example.com
```

After Certbot, verify that the public URL works:

```sh
curl -I https://vpn.example.com
```

## Firewall

Allow only the required public ports.

If you use a host firewall, allow:

```text
80/tcp
443/tcp
51820-51840/udp
```

Keep `8080/tcp` bound to `127.0.0.1` unless you are using a private reverse
proxy network.

If your WireGuard servers use different UDP ports, update the firewall and the
server configuration to match.

## First Web Setup

1. Open `https://vpn.example.com`.
2. Sign in with the initial admin username and password from
   `/etc/proidentity/proidentity.env`.
3. Change the admin password immediately.
4. Add VPN servers and assign each one a UDP port that is open on the firewall.
5. Add resources.
6. Add bundles.
7. Assign access by user:

```text
USER <> SERVERS <> BUNDLES <> RESOURCES
```

Users can then sign in from desktop or mobile clients, register their device,
and connect to assigned VPN servers.

## Backups

Back up at least:

```text
/etc/proidentity/config.yaml
/etc/proidentity/proidentity.env
MariaDB database proidentity
```

Database backup:

```sh
sudo mariadb-dump proidentity | gzip > proidentity-$(date +%Y%m%d-%H%M%S).sql.gz
```

Restore example:

```sh
gunzip -c proidentity-backup.sql.gz | sudo mariadb proidentity
```

Keep backups encrypted and off the server.

## Upgrade Server

Run the release updater:

```sh
sudo /opt/proidentity/update-release.sh
sudo systemctl status proidentity --no-pager
```

The updater compares `/opt/proidentity/VERSION` with the latest GitHub release,
downloads the server package when newer, verifies SHA256 checksums, backs up
the current binary and migrations under `/opt/proidentity/backups/`, and
restarts the service if it was running.

If the existing install does not yet have `/opt/proidentity/update-release.sh`,
download the updater once:

```sh
sudo curl -fsSL -o /tmp/proidentity-update-release.sh https://raw.githubusercontent.com/Pro-IT-Services/ProIdentity-Access/main/server/update-release.sh
sudo sh /tmp/proidentity-update-release.sh
```

The updater writes `/opt/proidentity/VERSION`, so future upgrades can use the
short command above.

Always take a database and `/etc/proidentity` backup before upgrading.

## Docker Deployment

Docker deployment is for teams that want Compose-managed MariaDB and server
containers. It does not use host networking. It still needs host TUN support,
`NET_ADMIN`, and published UDP ports.

Install Docker Engine and Compose plugin from Docker's official Debian
repository:

```sh
sudo apt update
sudo apt install -y ca-certificates curl git
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc

cat <<EOF | sudo tee /etc/apt/sources.list.d/docker.sources
Types: deb
URIs: https://download.docker.com/linux/debian
Suites: $(. /etc/os-release && echo "$VERSION_CODENAME")
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF

sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
```

Clone and start:

```sh
git clone https://github.com/Pro-IT-Services/ProIdentity-Access.git
cd ProIdentity-Access/server/docker
sudo ./host-prep.sh
./up.sh
```

On first run, `up.sh` creates `server/docker/.env` with random database, JWT,
and admin secrets. It prints the initial admin password once.

Important Docker `.env` fields:

| Variable | Required | Description |
| --- | --- | --- |
| `MYSQL_DATABASE` | Yes | MariaDB database name inside Docker. |
| `MYSQL_USER` | Yes | MariaDB app user. |
| `MYSQL_PASSWORD` | Yes | MariaDB app user password. |
| `MYSQL_ROOT_PASSWORD` | Yes | MariaDB root password. |
| `PROIDENTITY_IMAGE` | Optional | Image name for the locally built server image. |
| `PROIDENTITY_HTTP_BIND` | Yes | Host bind address for HTTP, usually `127.0.0.1`. |
| `PROIDENTITY_HTTP_PORT` | Yes | Host HTTP port, usually `8080`. |
| `PROIDENTITY_WG_UDP_PORTS` | Yes | Host UDP range published to the container. |
| `PROIDENTITY_JWT_SECRET` | Yes | Long random JWT signing secret. |
| `PROIDENTITY_TRUSTED_PROXIES` | Recommended | Reverse proxy networks allowed to pass client IP headers. |
| `WG_ADMIN_USER` | Yes on first boot | Initial admin username. |
| `WG_ADMIN_EMAIL` | Recommended | Initial admin email. |
| `WG_ADMIN_PASS` | Yes on first boot | Initial admin password. |

Docker commands:

```sh
cd ProIdentity-Access/server/docker
./logs.sh
./backup.sh
./down.sh
```

Reverse proxy Docker HTTP to:

```text
http://127.0.0.1:8080
```

Forward the selected WireGuard UDP range to the Docker host.

## Proxmox VM Installer

For Proxmox VE, use the VM installer when you want an interactive setup that
creates the VM first and configures the server on first boot.

Run from the Proxmox VE host shell as `root`:

```sh
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Pro-IT-Services/ProIdentity-Access/main/server/proxmox/proidentity-vm.sh)"
```

The wizard asks for VM resources, storage, network, release version, public URL,
HTTP listen mode, database settings, JWT secret, initial admin account, and SSH
account before it creates the VM.

It supports:

- direct mode without Nginx: `server.host` is set to `0.0.0.0`
- reverse-proxy mode: `server.host` is set to `127.0.0.1`
- generated DB user/password, JWT secret, admin password, and SSH password when
  those fields are left empty

See `server/proxmox/README.md` for details.

## Desktop Client Install

Download desktop installers from:

https://github.com/Pro-IT-Services/ProIdentity-Access/releases

Release assets:

```text
ProIdentity-Access-0.5.24.msi
ProIdentity-Access-0.5.24.pkg
ProIdentity-Access-0.5.24-SHA256SUMS.txt
```

Windows:

1. Run the MSI.
2. The installer adds the desktop app and a `LocalSystem` daemon service.
3. Launch ProIdentity Access.
4. Enter the server URL, for example `https://vpn.example.com`.
5. Register the device and sign in.

macOS:

1. Run the PKG.
2. The package installs the app and privileged LaunchDaemon.
3. Launch ProIdentity Access.
4. Enter the server URL, register the device, and sign in.

## Build From Source

Server requirements:

- Debian 13, Debian 12, or another Linux host with WireGuard support
- Go 1.24 or newer
- Node.js 22 or newer
- MariaDB

Build:

```sh
cd server/webui
npm ci
npm run build
cd ..
rm -rf internal/api/ui/dist
cp -r webui/dist internal/api/ui/dist
go build -trimpath -o bin/proidentity ./cmd/server
```

Windows desktop build requirements:

- Go 1.22 or newer
- Node.js 18 or newer
- Wails v2
- .NET SDK
- WiX v4

```powershell
cd client
powershell -ExecutionPolicy Bypass -File .\build.ps1 -Version 0.5.24
```

macOS desktop build requirements:

- macOS
- Xcode Command Line Tools
- Wails v2

```sh
cd client
./build.sh --version 0.5.24
```

Android build:

```sh
cd android
./build-frontend.sh
gradle assembleDebug
```

iOS build:

```sh
open ios/ProIdentity.xcodeproj
```

Use a physical iOS device with Network Extension capability for VPN testing.

## Security Notes

- Use HTTPS for every client-facing deployment.
- Keep `/etc/proidentity/proidentity.env`, `/etc/proidentity/config.yaml`, and
  `server/docker/.env` private and backed up securely.
- Rotate the initial admin password immediately.
- Restrict admin accounts.
- Publish only the WireGuard UDP ports you actually use.
- Do not expose the raw HTTP port directly to the internet.
- Verify `/dev/net/tun` support before deploying inside LXC or another
  restricted container environment.
- Back up the database before every upgrade.

## Troubleshooting

Check server service:

```sh
sudo systemctl status proidentity --no-pager
sudo journalctl -u proidentity -n 200 --no-pager
```

Check database access:

```sh
mariadb -u proidentity -p -h 127.0.0.1 proidentity
```

Check reverse proxy:

```sh
sudo nginx -t
sudo journalctl -u nginx -n 100 --no-pager
```

Check WireGuard tooling:

```sh
wg --version
ip link show
```

Check Docker deployment:

```sh
cd server/docker
docker compose ps
./logs.sh
```

## License

ProIdentity Access is free for personal, internal, and company use under the
ProIdentity Access Free Internal Use License 1.0. Redistribution, resale,
hosted-service/MSP/provider use, white-labeling, and sharing modified builds
require prior written permission from Pro-IT-Services. See `LICENSE` before
using, modifying, or deploying this project.
