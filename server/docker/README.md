# Docker Deployment

This deployment runs ProIdentity and WireGuard inside a Docker bridge network. It follows the same container pattern commonly used for WireGuard services: publish UDP ports, grant `NET_ADMIN`, mount `/dev/net/tun`, and enable forwarding sysctls.

Public repository:

- HTTPS: https://github.com/Pro-IT-Services/ProIdentity-Access
- SSH: git@github.com:Pro-IT-Services/ProIdentity-Access.git

## Files

- `docker-compose.yml` runs MariaDB and ProIdentity.
- `.env.example` lists required secrets and ports.
- `config.docker.yaml` is the container config file; secrets are supplied through environment variables.
- `up.sh`, `down.sh`, `logs.sh`, `backup.sh`, and `build.sh` manage the stack.
- `deploy-test.sh` and `deploy-test.ps1` copy this server tree to a test host and start it.

## First Run

```sh
cd server/docker
sudo ./host-prep.sh
./up.sh
```

If `.env` is missing, `up.sh` writes one with random secrets and prints the initial admin password.

The Web UI listens on `127.0.0.1:8080` by default. Put Caddy, Nginx, Traefik, or another company reverse proxy in front of that port.

WireGuard UDP ports `51820-51840` are published by default. In Docker deployments, keep managed WireGuard servers inside that range unless you also update `PROIDENTITY_WG_UDP_PORTS` and the Compose port mapping.

## Reverse Proxy

Proxy HTTPS traffic to:

```text
http://127.0.0.1:8080
```

Expose or forward UDP `51820-51840` to the Docker host.

## License

ProIdentity Access is source-available under the PolyForm Noncommercial
License 1.0.0 for noncommercial use. Commercial, enterprise, MSP, resale,
hosted-service, or other revenue-generating use requires a separate written
commercial license from Pro-IT-Services. See the repository root `LICENSE`.
