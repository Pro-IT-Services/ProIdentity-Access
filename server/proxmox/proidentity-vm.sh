#!/usr/bin/env bash
set -Eeuo pipefail

TITLE="ProIdentity Access VM Installer"
DEFAULT_REPO="Pro-IT-Services/ProIdentity-Access"
DEFAULT_VERSION="0.5.19"
DEFAULT_IMAGE_URL="https://cloud.debian.org/images/cloud/trixie/latest/debian-13-generic-amd64.qcow2"
SNIPPET_STORAGE="local"
SNIPPET_DIR="/var/lib/vz/snippets"
IMAGE_DIR="/var/lib/vz/template/iso"

die() {
  echo "ERROR: $*" >&2
  exit 1
}

cleanup_dialog() {
  clear || true
}
trap cleanup_dialog EXIT

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    die "run this script as root on the Proxmox VE host"
  fi
}

require_proxmox() {
  command -v qm >/dev/null 2>&1 || die "qm not found. Run this on a Proxmox VE host."
  command -v pvesm >/dev/null 2>&1 || die "pvesm not found. Run this on a Proxmox VE host."
}

ensure_dialog() {
  if command -v dialog >/dev/null 2>&1; then
    return
  fi
  echo "Installing dialog..."
  apt-get update
  apt-get install -y dialog
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

random_alnum() {
  local len="${1:-32}"
  local out=""
  while [[ "${#out}" -lt "$len" ]]; do
    out="${out}$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c "$((len - ${#out}))" || true)"
  done
  printf "%s" "$out"
}

shell_quote() {
  printf "%q" "$1"
}

msg() {
  dialog --title "$TITLE" --msgbox "$1" 14 78
}

input() {
  local label="$1"
  local default="${2:-}"
  local value
  value="$(dialog --title "$TITLE" --inputbox "$label" 10 78 "$default" 3>&1 1>&2 2>&3)" || exit 1
  printf "%s" "$value"
}

password_input() {
  local label="$1"
  local default="${2:-}"
  local value
  value="$(dialog --title "$TITLE" --insecure --passwordbox "$label" 10 78 "$default" 3>&1 1>&2 2>&3)" || exit 1
  printf "%s" "$value"
}

yesno() {
  local label="$1"
  dialog --title "$TITLE" --yesno "$label" 22 78
}

menu() {
  local label="$1"
  shift
  dialog --title "$TITLE" --menu "$label" 18 78 9 "$@" 3>&1 1>&2 2>&3
}

choose_storage() {
  local entries=()
  while read -r storage _; do
    [[ -z "$storage" || "$storage" == "Name" ]] && continue
    entries+=("$storage" "VM disk storage")
  done < <(pvesm status -content images 2>/dev/null | awk 'NR > 1 {print $1, $2}')
  if [[ "${#entries[@]}" -eq 0 ]]; then
    die "no Proxmox storage with image content found"
  fi
  menu "Select VM disk storage" "${entries[@]}"
}

choose_bridge() {
  local entries=()
  while read -r bridge; do
    entries+=("$bridge" "Linux bridge")
  done < <(ip -o link show | awk -F': ' '{print $2}' | cut -d@ -f1 | grep -E '^vmbr' | sort -u)
  if [[ "${#entries[@]}" -eq 0 ]]; then
    entries=("vmbr0" "Default bridge")
  fi
  menu "Select network bridge" "${entries[@]}"
}

validate_required() {
  local name="$1"
  local value="$2"
  [[ -n "$value" ]] || die "$name is required"
}

validate_vmid() {
  local vmid="$1"
  [[ "$vmid" =~ ^[0-9]+$ ]] || die "VM ID must be numeric"
  if qm status "$vmid" >/dev/null 2>&1; then
    die "VM ID $vmid already exists"
  fi
}

validate_identifier() {
  local name="$1"
  local value="$2"
  [[ "$value" =~ ^[A-Za-z0-9_]+$ ]] || die "$name may contain only letters, numbers, and underscores"
}

validate_linux_user() {
  local name="$1"
  local value="$2"
  [[ "$value" =~ ^[a-z_][a-z0-9_-]{0,31}$ ]] || die "$name must be a valid Linux username"
}

validate_secret_value() {
  local name="$1"
  local value="$2"
  [[ "$value" =~ ^[A-Za-z0-9_.-]+$ ]] || die "$name may contain only letters, numbers, dot, underscore, and dash"
}

validate_hostname() {
  local value="$1"
  [[ "$value" =~ ^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$ ]] || die "guest hostname must be a valid single-label hostname"
}

validate_snippets_storage() {
  if ! pvesm status -content snippets 2>/dev/null | awk '{print $1}' | grep -qx "$SNIPPET_STORAGE"; then
    die "Proxmox storage '$SNIPPET_STORAGE' must allow snippets content for cloud-init. Enable snippets on local storage or edit SNIPPET_STORAGE in this script."
  fi
}

write_firstboot_snippet() {
  local snippet="$1"
  local db_name_q db_user_q db_pass_q admin_user_q admin_pass_q admin_email_q
  local jwt_secret_q public_url_q repo_q version_q server_host_q server_port_q cors_extra_q
  local hostname_q ci_user_q ci_password_q
  hostname_q="$(shell_quote "$HOSTNAME")"
  db_name_q="$(shell_quote "$DB_NAME")"
  db_user_q="$(shell_quote "$DB_USER")"
  db_pass_q="$(shell_quote "$DB_PASS")"
  admin_user_q="$(shell_quote "$ADMIN_USER")"
  admin_pass_q="$(shell_quote "$ADMIN_PASS")"
  admin_email_q="$(shell_quote "$ADMIN_EMAIL")"
  jwt_secret_q="$(shell_quote "$JWT_SECRET")"
  public_url_q="$(shell_quote "$PUBLIC_URL")"
  repo_q="$(shell_quote "$REPO")"
  version_q="$(shell_quote "$VERSION")"
  server_host_q="$(shell_quote "$SERVER_HOST")"
  server_port_q="$(shell_quote "$SERVER_PORT")"
  cors_extra_q="$(shell_quote "$CORS_EXTRA")"
  ci_user_q="$(shell_quote "$CI_USER")"
  ci_password_q="$(shell_quote "$CI_PASSWORD")"

  cat >"$snippet" <<EOF
#cloud-config
hostname: ${HOSTNAME}
manage_etc_hosts: true
ssh_pwauth: true
package_update: true
package_upgrade: true
packages:
  - ca-certificates
  - curl
  - tar
  - gzip
  - openssl
  - coreutils
  - mariadb-server
  - mariadb-client
  - wireguard-tools
  - iproute2
  - iptables
  - nftables
  - procps
  - qemu-guest-agent
  - openssh-server
  - sudo
write_files:
  - path: /root/proidentity-firstboot.sh
    owner: root:root
    permissions: '0700'
    content: |
      #!/usr/bin/env bash
      set -Eeuo pipefail

      DB_NAME=${db_name_q}
      DB_USER=${db_user_q}
      DB_PASS=${db_pass_q}
      ADMIN_USER=${admin_user_q}
      ADMIN_PASS=${admin_pass_q}
      ADMIN_EMAIL=${admin_email_q}
      JWT_SECRET=${jwt_secret_q}
      PUBLIC_URL=${public_url_q}
      REPO=${repo_q}
      VERSION=${version_q}
      SERVER_HOST=${server_host_q}
      SERVER_PORT=${server_port_q}
      CORS_EXTRA=${cors_extra_q}
      HOSTNAME=${hostname_q}
      CI_USER=${ci_user_q}
      CI_PASSWORD=${ci_password_q}

      sql_escape() {
        printf "%s" "\$1" | sed "s/'/''/g"
      }

      systemctl enable --now qemu-guest-agent || true
      systemctl enable --now mariadb
      systemctl enable --now ssh || true
      hostnamectl set-hostname "\$HOSTNAME" || true

      if ! id -u "\$CI_USER" >/dev/null 2>&1; then
        useradd -m -s /bin/bash -G sudo "\$CI_USER"
      fi
      printf "%s:%s\n" "\$CI_USER" "\$CI_PASSWORD" | chpasswd
      install -d -m 0755 /etc/sudoers.d
      printf "%s ALL=(ALL) NOPASSWD:ALL\n" "\$CI_USER" >/etc/sudoers.d/90-proidentity-cloud-user
      chmod 0440 /etc/sudoers.d/90-proidentity-cloud-user

      cat >/etc/sysctl.d/99-proidentity.conf <<'SYSCTL'
      net.ipv4.ip_forward=1
      net.ipv4.conf.all.src_valid_mark=1
      SYSCTL
      sysctl --system >/dev/null || true

      db_name_esc="\$(sql_escape "\$DB_NAME")"
      db_user_esc="\$(sql_escape "\$DB_USER")"
      db_pass_esc="\$(sql_escape "\$DB_PASS")"

      mariadb <<SQL
      CREATE DATABASE IF NOT EXISTS \\\`\$db_name_esc\\\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
      CREATE USER IF NOT EXISTS '\$db_user_esc'@'localhost' IDENTIFIED BY '\$db_pass_esc';
      ALTER USER '\$db_user_esc'@'localhost' IDENTIFIED BY '\$db_pass_esc';
      GRANT ALL PRIVILEGES ON \\\`\$db_name_esc\\\`.* TO '\$db_user_esc'@'localhost';
      FLUSH PRIVILEGES;
      SQL

      curl -fsSL -o /root/proidentity-install-release.sh "https://raw.githubusercontent.com/\$REPO/main/server/install-release.sh"
      sh /root/proidentity-install-release.sh "\$REPO" "\$VERSION"

      install -d -m 0750 /etc/proidentity
      cat >/etc/proidentity/proidentity.env <<ENV
      PROIDENTITY_JWT_SECRET=\$JWT_SECRET
      PROIDENTITY_DATABASE_DSN=\$DB_USER:\$DB_PASS@tcp(127.0.0.1:3306)/\$DB_NAME
      PROIDENTITY_SERVER_HOST=\$SERVER_HOST
      PROIDENTITY_SERVER_PORT=\$SERVER_PORT
      WG_ADMIN_USER=\$ADMIN_USER
      WG_ADMIN_PASS=\$ADMIN_PASS
      WG_ADMIN_EMAIL=\$ADMIN_EMAIL
      ENV

      cors_block="    - \\"\$PUBLIC_URL\\""
      if [ -n "\$CORS_EXTRA" ]; then
        cors_block="\$cors_block
          - \\"\$CORS_EXTRA\\""
      fi

      cat >/etc/proidentity/config.yaml <<CONFIG
      server:
        host: "\$SERVER_HOST"
        port: \$SERVER_PORT
        cors_origins:
      \$cors_block

      database:
        dsn: "\$DB_USER:\$DB_PASS@tcp(127.0.0.1:3306)/\$DB_NAME"

      auth:
        jwt_secret: "\$JWT_SECRET"
      CONFIG

      chown -R root:root /etc/proidentity
      chmod 0750 /etc/proidentity
      chmod 0640 /etc/proidentity/config.yaml
      chmod 0600 /etc/proidentity/proidentity.env

      systemctl daemon-reload
      systemctl enable proidentity
      systemctl restart proidentity

      cat >/root/proidentity-install-summary.txt <<SUMMARY
      ProIdentity Access setup complete.

      URL: \$PUBLIC_URL
      Server bind: \$SERVER_HOST:\$SERVER_PORT
      Admin user: \$ADMIN_USER
      Admin password: \$ADMIN_PASS
      SSH user: \$CI_USER
      SSH password: \$CI_PASSWORD
      Config: /etc/proidentity/config.yaml
      Env: /etc/proidentity/proidentity.env

      Check status:
        systemctl status proidentity --no-pager
        journalctl -u proidentity -n 100 --no-pager
      SUMMARY
runcmd:
  - [bash, -lc, "/root/proidentity-firstboot.sh 2>&1 | tee /root/proidentity-firstboot.log"]
final_message: "ProIdentity Access VM setup finished. See /root/proidentity-install-summary.txt"
EOF
}

download_image() {
  mkdir -p "$IMAGE_DIR"
  local image_path="$IMAGE_DIR/proidentity-debian-13-generic-amd64.qcow2"
  if [[ ! -s "$image_path" ]]; then
    msg "The Debian 13 cloud image will be downloaded now. This can take a few minutes."
    curl -fL "$IMAGE_URL" -o "$image_path"
  fi
  printf "%s" "$image_path"
}

create_vm() {
  local image_path="$1"
  local net0="virtio,bridge=${BRIDGE}"
  if [[ -n "$VLAN_TAG" ]]; then
    net0="${net0},tag=${VLAN_TAG}"
  fi

  qm create "$VMID" \
    --name "$VM_NAME" \
    --ostype l26 \
    --memory "$MEMORY_MB" \
    --cores "$CORES" \
    --cpu host \
    --agent enabled=1 \
    --scsihw virtio-scsi-pci \
    --net0 "$net0" \
    --serial0 socket \
    --vga serial0

  qm importdisk "$VMID" "$image_path" "$STORAGE" >/dev/null
  local imported_disk
  imported_disk="$(qm config "$VMID" | awk '/^unused0:/ {print $2; exit}')"
  [[ -n "$imported_disk" ]] || die "could not find imported disk on VM $VMID"
  qm set "$VMID" --scsi0 "${imported_disk},discard=on,ssd=1" >/dev/null
  qm set "$VMID" --delete unused0 >/dev/null
  qm set "$VMID" --ide2 "${STORAGE}:cloudinit" >/dev/null
  qm set "$VMID" --boot order=scsi0 >/dev/null
  qm set "$VMID" --bootdisk scsi0 >/dev/null
  qm set "$VMID" --nameserver "$DNS_SERVER" >/dev/null
  qm set "$VMID" --cicustom "user=${SNIPPET_STORAGE}:snippets/proidentity-vm-${VMID}-user.yml" >/dev/null

  if [[ "$IP_MODE" == "dhcp" ]]; then
    qm set "$VMID" --ipconfig0 ip=dhcp >/dev/null
  else
    qm set "$VMID" --ipconfig0 "ip=${IP_CIDR},gw=${GATEWAY}" >/dev/null
  fi

  qm resize "$VMID" scsi0 "${DISK_GB}G" >/dev/null
}

collect_inputs() {
  msg "This wizard creates a Debian 13 VM on this Proxmox host, then configures ProIdentity Access on first boot with cloud-init.\n\nAll required server, database, admin, listen, and network settings are collected before the VM is created."

  VMID="$(input "VM ID" "120")"
  validate_vmid "$VMID"
  VM_NAME="$(input "VM name" "proidentity-access")"
  validate_required "VM name" "$VM_NAME"
  HOSTNAME="$(input "Guest hostname" "proidentity-access")"
  validate_required "Guest hostname" "$HOSTNAME"
  validate_hostname "$HOSTNAME"

  STORAGE="$(choose_storage)"
  BRIDGE="$(choose_bridge)"
  VLAN_TAG="$(input "VLAN tag. Leave empty for none." "")"
  DISK_GB="$(input "Disk size in GB" "32")"
  CORES="$(input "CPU cores" "2")"
  MEMORY_MB="$(input "Memory in MB" "4096")"

  IP_MODE="$(menu "Guest network mode" "dhcp" "Use DHCP" "static" "Set static IP")"
  if [[ "$IP_MODE" == "static" ]]; then
    IP_CIDR="$(input "Static IP with CIDR, for example 10.10.2.69/24" "")"
    GATEWAY="$(input "Gateway, for example 10.10.2.1" "")"
    validate_required "Static IP" "$IP_CIDR"
    validate_required "Gateway" "$GATEWAY"
  else
    IP_CIDR=""
    GATEWAY=""
  fi
  DNS_SERVER="$(input "DNS server" "1.1.1.1")"

  REPO="$(input "GitHub repository owner/name" "$DEFAULT_REPO")"
  VERSION="$(input "ProIdentity release version" "$DEFAULT_VERSION")"
  IMAGE_URL="$(input "Debian 13 cloud image URL" "$DEFAULT_IMAGE_URL")"
  PUBLIC_URL="$(input "Public or direct server URL, for example https://vpn.example.com or http://10.10.2.69:8080" "http://10.10.2.69:8080")"
  SERVER_PORT="$(input "ProIdentity HTTP port" "8080")"

  LISTEN_MODE="$(menu "HTTP listen mode" \
    "direct" "No Nginx: listen on 0.0.0.0" \
    "proxy" "Reverse proxy: listen on 127.0.0.1")"
  if [[ "$LISTEN_MODE" == "direct" ]]; then
    SERVER_HOST="0.0.0.0"
  else
    SERVER_HOST="127.0.0.1"
  fi
  CORS_EXTRA="$(input "Extra CORS origin. Leave empty if not needed." "")"

  DB_NAME="$(input "MariaDB database name. Leave empty for proidentity." "proidentity")"
  DB_USER="$(input "MariaDB app user. Leave empty to generate." "proidentity")"
  DB_PASS="$(password_input "MariaDB app password. Leave empty to generate." "")"
  JWT_SECRET="$(password_input "JWT secret. Leave empty to generate." "")"
  ADMIN_USER="$(input "Initial admin username" "admin")"
  ADMIN_EMAIL="$(input "Initial admin email" "admin@localhost")"
  ADMIN_PASS="$(password_input "Initial admin password. Leave empty to generate." "")"

  CI_USER="$(input "Guest SSH/cloud-init user" "proidentity")"
  CI_PASSWORD="$(password_input "Guest SSH password. Leave empty to generate." "")"

  if [[ -z "$DB_NAME" ]]; then DB_NAME="proidentity"; fi
  if [[ -z "$DB_USER" ]]; then DB_USER="proidentity_$(random_alnum 8 | tr 'A-Z' 'a-z')"; fi
  if [[ -z "$DB_PASS" ]]; then DB_PASS="$(random_alnum 32)"; fi
  if [[ -z "$JWT_SECRET" ]]; then JWT_SECRET="$(random_alnum 64)"; fi
  if [[ -z "$ADMIN_PASS" ]]; then ADMIN_PASS="$(random_alnum 24)"; fi
  if [[ -z "$CI_PASSWORD" ]]; then CI_PASSWORD="$(random_alnum 24)"; fi

  validate_required "Repository" "$REPO"
  validate_required "Version" "$VERSION"
  validate_required "Public URL" "$PUBLIC_URL"
  validate_required "Server port" "$SERVER_PORT"
  validate_required "Database name" "$DB_NAME"
  validate_required "Database user" "$DB_USER"
  validate_identifier "Database name" "$DB_NAME"
  validate_identifier "Database user" "$DB_USER"
  validate_required "Database password" "$DB_PASS"
  validate_required "JWT secret" "$JWT_SECRET"
  validate_required "Admin user" "$ADMIN_USER"
  validate_required "Admin password" "$ADMIN_PASS"
  validate_required "Guest SSH user" "$CI_USER"
  validate_required "Guest SSH password" "$CI_PASSWORD"
  validate_secret_value "Database password" "$DB_PASS"
  validate_secret_value "JWT secret" "$JWT_SECRET"
  validate_identifier "Admin username" "$ADMIN_USER"
  validate_secret_value "Admin password" "$ADMIN_PASS"
  validate_linux_user "Guest SSH user" "$CI_USER"
  validate_secret_value "Guest SSH password" "$CI_PASSWORD"

  local summary
  summary="VM ID: $VMID
VM name: $VM_NAME
Storage: $STORAGE
Bridge: $BRIDGE
Network: $IP_MODE ${IP_CIDR:-}
Disk: ${DISK_GB}G
CPU/RAM: ${CORES} cores / ${MEMORY_MB} MB
Release: $REPO $VERSION
URL: $PUBLIC_URL
Listen: $SERVER_HOST:$SERVER_PORT
Database: $DB_NAME as $DB_USER
Initial admin: $ADMIN_USER

Create and start the VM now?"

  yesno "$summary" || exit 1
}

main() {
  require_root
  require_proxmox
  ensure_dialog
  validate_snippets_storage
  require_cmd curl
  require_cmd ip
  require_cmd openssl

  collect_inputs

  mkdir -p "$SNIPPET_DIR"
  local snippet="$SNIPPET_DIR/proidentity-vm-${VMID}-user.yml"
  write_firstboot_snippet "$snippet"

  local image_path
  image_path="$(download_image)"
  create_vm "$image_path"

  qm set "$VMID" --description "ProIdentity Access server. First boot log: /root/proidentity-firstboot.log" >/dev/null
  qm start "$VMID"

  msg "VM $VMID has been created and started.\n\nFirst boot will install MariaDB, WireGuard tools, ProIdentity Access $VERSION, write all config/env values, and start the service.\n\nGuest setup log:\n/root/proidentity-firstboot.log\n\nSummary file:\n/root/proidentity-install-summary.txt\n\nInitial admin: $ADMIN_USER\nInitial admin password: $ADMIN_PASS\n\nSSH user: $CI_USER\nSSH password: $CI_PASSWORD"
}

main "$@"
