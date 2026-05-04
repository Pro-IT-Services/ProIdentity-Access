# ProIdentity Access Server

This directory contains the Go management server, admin Web UI, database
migrations, WireGuard session manager, release installer, systemd service, and
Docker deployment files.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

## Recommended Platform

Recommended production OS:

- Debian 13 "Trixie" `amd64`

Recommended production deployment:

- GitHub Release archive
- `server/install-release.sh`
- MariaDB
- systemd
- TLS reverse proxy
- WireGuard UDP ports opened on the firewall

Docker Compose deployment is also available in `server/docker`, but it builds
the image locally from a checked-out repository. Use the release deployment for
the simplest binary install.

## Required Debian 13 Packages

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

Enable services and host networking:

```sh
sudo systemctl enable --now mariadb
sudo systemctl enable --now nginx

cat <<'EOF' | sudo tee /etc/sysctl.d/99-proidentity.conf
net.ipv4.ip_forward=1
net.ipv4.conf.all.src_valid_mark=1
EOF

sudo sysctl --system
test -c /dev/net/tun && echo "TUN device is available"
```

## Database Setup

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

Use this DSN in `/etc/proidentity/config.yaml`:

```text
proidentity:change-this-db-password@tcp(127.0.0.1:3306)/proidentity
```

## Install From Release

```sh
curl -fsSL -o install-release.sh \
  https://raw.githubusercontent.com/Pro-IT-Services/ProIdentity-Access/main/server/install-release.sh
sudo sh install-release.sh Pro-IT-Services/ProIdentity-Access 0.5.23
```

Release assets downloaded by the installer:

```text
ProIdentity-Access-Server-0.5.23-linux-amd64.tar.gz
ProIdentity-Access-0.5.23-SHA256SUMS.txt
```

When `/etc/proidentity/proidentity.env` does not exist, the installer generates
an admin password, JWT secret, database user, database password, and database
DSN. It prints the generated values and SQL you can run if the database and user
do not already exist.

Installed paths:

| Path | Purpose |
| --- | --- |
| `/opt/proidentity/bin/proidentity` | Server binary |
| `/opt/proidentity/migrations/` | Database migrations |
| `/etc/proidentity/config.yaml` | Main config |
| `/etc/proidentity/proidentity.env` | Secret env file |
| `/opt/proidentity/VERSION` | Installed server release version |
| `/opt/proidentity/update-release.sh` | Release updater script |
| `/etc/systemd/system/proidentity.service` | systemd service |

## Configuration

Edit:

```sh
sudo nano /etc/proidentity/config.yaml
sudo nano /etc/proidentity/proidentity.env
```

Minimal `/etc/proidentity/config.yaml`:

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
| `server.port` | Yes | Backend HTTP port. |
| `server.cors_origins` | Yes | Public HTTPS URL allowed by browser clients. |
| `database.dsn` | Yes | MariaDB DSN. |
| `auth.jwt_secret` | Yes | JWT signing secret. `PROIDENTITY_JWT_SECRET` overrides this value when set. |

Env fields:

| Variable | Required | Description |
| --- | --- | --- |
| `PROIDENTITY_JWT_SECRET` | Yes | Long random JWT signing secret. |
| `PROIDENTITY_DATABASE_DSN` | Optional | Overrides `database.dsn` from `config.yaml`. |
| `PROIDENTITY_DATABASE_NAME` | Optional install input | Used by `install-release.sh` when generating a DSN. |
| `PROIDENTITY_DATABASE_USER` | Optional install input | Used by `install-release.sh` when generating a DSN. |
| `PROIDENTITY_DATABASE_PASS` | Optional install input | Used by `install-release.sh` when generating a DSN. |
| `PROIDENTITY_DATABASE_HOST` | Optional install input | Defaults to `127.0.0.1:3306` when generating a DSN. |
| `PROIDENTITY_SERVER_HOST` | Optional | Overrides `server.host` from `config.yaml`. |
| `PROIDENTITY_SERVER_PORT` | Optional | Overrides `server.port` from `config.yaml`. |
| `WG_ADMIN_USER` | Yes on first boot | Initial admin username. |
| `WG_ADMIN_PASS` | Yes on first boot | Initial admin password. |
| `WG_ADMIN_EMAIL` | Recommended | Initial admin email. |

Generate secrets:

```sh
openssl rand -base64 48
```

Secure config files:

```sh
sudo chown -R root:root /etc/proidentity
sudo chmod 0750 /etc/proidentity
sudo chmod 0640 /etc/proidentity/config.yaml
sudo chmod 0600 /etc/proidentity/proidentity.env
```

## Start And Verify

```sh
sudo systemctl daemon-reload
sudo systemctl start proidentity
sudo systemctl status proidentity --no-pager
sudo journalctl -u proidentity -n 100 --no-pager
```

The server applies migrations automatically on startup.

Local HTTP check:

```sh
curl -i http://127.0.0.1:8080/
```

## Reverse Proxy

Example Nginx site:

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

Enable TLS:

```sh
sudo ln -s /etc/nginx/sites-available/proidentity /etc/nginx/sites-enabled/proidentity
sudo nginx -t
sudo systemctl reload nginx
sudo certbot --nginx -d vpn.example.com
```

Expose only:

```text
80/tcp
443/tcp
selected WireGuard UDP ports, for example 51820-51840/udp
```

## First Admin Setup

1. Open `https://vpn.example.com`.
2. Sign in with the initial admin account from `/etc/proidentity/proidentity.env`.
3. Change the admin password.
4. Add WireGuard servers.
5. Add resources and bundles.
6. Assign access by user:

```text
USER <> SERVERS <> BUNDLES <> RESOURCES
```

## Backups

Back up:

```text
/etc/proidentity/config.yaml
/etc/proidentity/proidentity.env
MariaDB database proidentity
```

Database backup:

```sh
sudo mariadb-dump proidentity | gzip > proidentity-$(date +%Y%m%d-%H%M%S).sql.gz
```

## Upgrade

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

Back up the database and `/etc/proidentity` before every upgrade.

## Docker

Docker deployment files are in `server/docker`.

```sh
git clone https://github.com/Pro-IT-Services/ProIdentity-Access.git
cd ProIdentity-Access/server/docker
sudo ./host-prep.sh
./up.sh
```

See `server/docker/README.md` for Docker environment variables and Compose
operations.

## Proxmox VM Installer

For Proxmox VE, run the interactive VM installer from the Proxmox host shell:

```sh
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Pro-IT-Services/ProIdentity-Access/main/server/proxmox/proidentity-vm.sh)"
```

The wizard collects all required values before creating the VM, including VM
resources, storage, network, database credentials, JWT secret, admin account,
release version, and HTTP listen mode.

Use direct mode if you do not want Nginx:

```text
server.host = 0.0.0.0
```

Use reverse-proxy mode if another system terminates TLS:

```text
server.host = 127.0.0.1
```

See `server/proxmox/README.md`.

## Build From Source

Requirements:

- Go 1.24 or newer
- Node.js 22 or newer
- MariaDB
- Linux with WireGuard tools, `iproute2`, firewall tooling, and TUN support

```sh
cd webui
npm ci
npm run build
cd ..
rm -rf internal/api/ui/dist
cp -r webui/dist internal/api/ui/dist
go build -trimpath -o bin/proidentity ./cmd/server
```

Run with a configured database:

```sh
export WG_ADMIN_USER=admin
export WG_ADMIN_PASS='change-this-password'
export WG_ADMIN_EMAIL=admin@example.com
./bin/proidentity config.yaml
```

## Troubleshooting

```sh
sudo systemctl status proidentity --no-pager
sudo journalctl -u proidentity -n 200 --no-pager
mariadb -u proidentity -p -h 127.0.0.1 proidentity
wg --version
ip link show
```

## License

ProIdentity Access is free for personal, internal, and company use under the
ProIdentity Access Free Internal Use License 1.0. Redistribution, resale,
hosted-service/MSP/provider use, white-labeling, and sharing modified builds
require prior written permission from Pro-IT-Services. See the repository root
`LICENSE`.
