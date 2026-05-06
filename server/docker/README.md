# Docker Deployment

This deployment builds the ProIdentity server image locally from this
repository and runs ProIdentity, MariaDB, and WireGuard through Docker Compose
inside a Docker bridge network.

It does not require Docker host networking. The host must still provide:

- `/dev/net/tun`
- `NET_ADMIN` capability for the ProIdentity container
- UDP port forwarding for the WireGuard ports you assign

For production installs that do not need Docker, the recommended path is the
GitHub Release archive and `server/install-release.sh` documented in the root
README.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

## Recommended Host

- Debian 13 "Trixie" `amd64`
- Docker Engine from Docker's official Debian repository
- Docker Compose plugin, used as `docker compose`

## Install Docker On Debian 13

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
docker compose version
```

## Files

| File | Purpose |
| --- | --- |
| `docker-compose.yml` | Runs MariaDB and ProIdentity. |
| `.env.example` | Lists required secrets and ports. |
| `config.docker.yaml` | Server config used inside the container. |
| `host-prep.sh` | Enables host TUN and forwarding requirements. |
| `up.sh` | Creates `.env` if missing, builds, and starts the stack. |
| `down.sh` | Stops the stack. |
| `logs.sh` | Follows container logs. |
| `backup.sh` | Creates database backup output from the Docker database. |
| `build.sh` | Builds the server image. |

## First Run

```sh
git clone https://github.com/Pro-IT-Services/ProIdentity-Access.git
cd ProIdentity-Access/server/docker
sudo ./host-prep.sh
./up.sh
```

On first run, `up.sh` creates `.env` with random secrets and prints the initial
admin password once.

The Web UI listens on `127.0.0.1:8080` by default. Put Caddy, Nginx, Traefik,
or another company reverse proxy in front of that port.

WireGuard UDP ports `51820-51840` are published by default. Keep managed
WireGuard servers inside that range unless you also update
`PROIDENTITY_WG_UDP_PORTS` and the Compose port mapping.

## Environment Variables

Edit `.env` before first production start if you need custom values:

```sh
nano .env
```

| Variable | Required | Default/example | Description |
| --- | --- | --- | --- |
| `MYSQL_DATABASE` | Yes | `proidentity` | MariaDB database name. |
| `MYSQL_USER` | Yes | `proidentity` | MariaDB app user. |
| `MYSQL_PASSWORD` | Yes | generated or placeholder | Password for `MYSQL_USER`. |
| `MYSQL_ROOT_PASSWORD` | Yes | generated or placeholder | MariaDB root password. |
| `PROIDENTITY_IMAGE` | Optional | `proidentity/server:local` | Local image name used by Compose. |
| `PROIDENTITY_HTTP_BIND` | Yes | `127.0.0.1` | Host interface for HTTP publishing. |
| `PROIDENTITY_HTTP_PORT` | Yes | `8080` | Host HTTP port for reverse proxy. |
| `PROIDENTITY_WG_UDP_PORTS` | Yes | `51820-51840` | Host UDP range mapped to container `51820-51840/udp`. |
| `PROIDENTITY_JWT_SECRET` | Yes | generated or placeholder | Long random JWT signing secret. |
| `PROIDENTITY_TRUSTED_PROXIES` | Recommended | private networks | Reverse proxy IPs/CIDRs allowed to pass client IP headers. |
| `PROIDENTITY_TRUST_LOOPBACK_PROXY` | Recommended | `1` | Trust proxy headers from same-host reverse proxies. |
| `PROIDENTITY_DISABLE_X_FORWARDED_FOR` | Optional | empty | Set to `1` to ignore `X-Forwarded-For`. |
| `WG_ADMIN_USER` | Yes on first boot | `admin` | Initial admin username. |
| `WG_ADMIN_EMAIL` | Recommended | `admin@localhost` | Initial admin email. |
| `WG_ADMIN_PASS` | Yes on first boot | generated or placeholder | Initial admin password. |

Generate replacement secrets:

```sh
openssl rand -base64 48
```

## Ports

Default published ports:

| Port | Purpose |
| --- | --- |
| `127.0.0.1:8080/tcp` | Web UI and API, intended for reverse proxy only. |
| `51820-51840/udp` | WireGuard tunnel traffic. |

If you change `PROIDENTITY_WG_UDP_PORTS`, also adjust the `ports` mapping in
`docker-compose.yml` if the container-side range changes.

## Reverse Proxy

Proxy HTTPS traffic to:

```text
http://127.0.0.1:8080
```

Nginx example:

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

Expose or forward UDP `51820-51840` to the Docker host.

## Operations

```sh
cd ProIdentity-Access/server/docker
./logs.sh
./backup.sh
./down.sh
./up.sh
```

Direct Compose commands:

```sh
docker compose ps
docker compose logs -f proidentity
docker compose restart proidentity
```

## Backup

Back up:

- `.env`
- Docker volumes `proidentity_db_data` and `proidentity_app_data`
- Database dump from `./backup.sh`

Keep backups encrypted and off the Docker host.

## Troubleshooting

Check TUN:

```sh
test -c /dev/net/tun && echo "TUN device is available"
```

Check stack:

```sh
docker compose ps
docker compose logs --tail=200 proidentity
docker compose logs --tail=200 db
```

Check host UDP listening/publishing:

```sh
docker compose port proidentity 51820/udp
```

## License

ProIdentity Access is free for personal, internal, and company use under the
ProIdentity Access Free Internal Use License 1.0. Redistribution, resale,
hosted-service/MSP/provider use, white-labeling, and sharing modified builds
require prior written permission from Pro-IT-Services. See the repository root
`LICENSE`.
