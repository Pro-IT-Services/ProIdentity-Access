# Proxmox VM Installer

`proidentity-vm.sh` creates a Debian 13 VM on a Proxmox VE host and installs
ProIdentity Access inside the guest on first boot.

Run it from the Proxmox VE host shell as `root`:

```sh
bash -c "$(curl -fsSL https://raw.githubusercontent.com/Pro-IT-Services/ProIdentity-Access/main/server/proxmox/proidentity-vm.sh)"
```

The script uses `dialog` and asks for all required values before it creates the
VM:

- VM ID, name, storage, bridge, VLAN, disk, CPU, and memory
- DHCP or static guest IP settings
- ProIdentity release repository and version
- public URL
- direct `0.0.0.0` listening or reverse-proxy `127.0.0.1` listening
- HTTP port
- MariaDB database name, user, and password
- JWT secret
- initial admin username, password, and email
- guest SSH username and password

If database user, database password, JWT secret, admin password, or SSH password
are left empty, the script generates safe random values and shows them in the
final summary.

The VM uses Debian 13 cloud-init and runs a first-boot setup script that:

- installs MariaDB, WireGuard tools, qemu guest agent, SSH, and sudo
- creates the database and database user
- downloads and verifies the ProIdentity Access release archive
- writes `/etc/proidentity/config.yaml`
- writes `/etc/proidentity/proidentity.env`
- enables IP forwarding
- enables and starts `proidentity.service`

Setup logs inside the VM:

```text
/root/proidentity-firstboot.log
/root/proidentity-install-summary.txt
```

## Direct Mode

Choose direct mode when you do not want Nginx or another reverse proxy in front
of the server. The script writes:

```yaml
server:
  host: "0.0.0.0"
  port: 8080
```

Use a URL such as:

```text
http://10.10.2.69:8080
```

## Reverse Proxy Mode

Choose reverse proxy mode when another system terminates TLS and forwards HTTP
to the VM. The script writes:

```yaml
server:
  host: "127.0.0.1"
  port: 8080
```

Proxy HTTPS traffic to:

```text
http://<vm-ip>:8080
```

## Notes

- This is VM-first because WireGuard in LXC depends on host-specific TUN and
  capability settings.
- The Proxmox `local` storage must allow `snippets` content for the cloud-init
  user-data file.
- The VM must be able to reach GitHub and Debian package repositories on first
  boot.
