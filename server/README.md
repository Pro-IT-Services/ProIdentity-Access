# ProIdentity Access Server

This directory contains the Go management server, admin Web UI, database
migrations, WireGuard session manager, and Docker deployment files.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

## Recommended Release Deployment

Use the release archive for production deployments. It includes:

- `bin/proidentity`
- `migrations/`
- `config.example.yaml`
- `systemd/proidentity.service`

Create a MariaDB database and user:

```sql
CREATE DATABASE proidentity CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'proidentity'@'localhost' IDENTIFIED BY 'change-this-db-password';
GRANT ALL PRIVILEGES ON proidentity.* TO 'proidentity'@'localhost';
FLUSH PRIVILEGES;
```

Install from a GitHub Release:

```sh
curl -fsSL -o install-release.sh \
  https://raw.githubusercontent.com/Pro-IT-Services/ProIdentity-Access/main/server/install-release.sh
sudo sh install-release.sh Pro-IT-Services/ProIdentity-Access 0.5.19
```

Edit `/etc/proidentity/config.yaml`, then start the service:

```sh
sudo systemctl start proidentity
sudo systemctl status proidentity --no-pager
```

The server applies migrations automatically during startup.

## Docker

The Docker setup builds the server image locally from this checked-out source
tree and runs it with MariaDB through Docker Compose. Use the release archive
above if you want the recommended binary deployment instead of a local Docker
build.

```sh
git clone https://github.com/Pro-IT-Services/ProIdentity-Access.git
cd ProIdentity-Access/server
cd docker
sudo ./host-prep.sh
./up.sh
```

The Docker setup publishes the HTTP service on `127.0.0.1:8080` by default and
publishes WireGuard UDP ports `51820-51840`. Put a reverse proxy with TLS in
front of the HTTP service for production.

## Build From Source

Requirements:

- Go 1.24 or newer
- Node.js 22 or newer
- MariaDB

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

Migrations in `migrations/` are applied automatically during startup.

## License

ProIdentity Access is source-available under the PolyForm Noncommercial
License 1.0.0 for noncommercial use. Commercial, enterprise, MSP, resale,
hosted-service, or other revenue-generating use requires a separate written
commercial license from Pro-IT-Services. See the repository root `LICENSE`.
