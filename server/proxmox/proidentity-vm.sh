#!/usr/bin/env bash
set -Eeuo pipefail

TITLE="ProIdentity Access VM Installer"
DEFAULT_REPO="Pro-IT-Services/ProIdentity-Access"
DEFAULT_VERSION="0.5.20"
DEFAULT_IMAGE_URL="https://cloud.debian.org/images/cloud/trixie/latest/debian-13-generic-amd64.qcow2"
DEFAULT_SNIPPET_STORAGE="local"
SNIPPET_STORAGE=""
SNIPPET_DIR="/var/lib/vz/snippets"
IMAGE_DIR="/var/lib/vz/template/iso"

if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
  LOG_FILE="${PROIDENTITY_VM_LOG:-/root/proidentity-vm-installer-$(date +%Y%m%d-%H%M%S).log}"
else
  LOG_FILE="${PROIDENTITY_VM_LOG:-/tmp/proidentity-vm-installer-$(date +%Y%m%d-%H%M%S).log}"
fi
mkdir -p "$(dirname "$LOG_FILE")"
touch "$LOG_FILE"

log() {
  printf '%s %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" >>"$LOG_FILE"
}

die() {
  local message="$*"
  echo "ERROR: $message" >&2
  echo "Log file: $LOG_FILE" >&2
  log "ERROR: $message"
  if command -v dialog >/dev/null 2>&1; then
    dialog --title "$TITLE" --msgbox "ERROR:\n$message\n\nLog file:\n$LOG_FILE" 12 78 || true
  fi
  exit 1
}

on_error() {
  local rc="$?"
  local line="${1:-unknown}"
  local cmd="${BASH_COMMAND:-unknown}"
  echo "" >&2
  echo "ERROR: command failed with exit code $rc at line $line" >&2
  echo "Command: $cmd" >&2
  echo "Log file: $LOG_FILE" >&2
  log "ERROR: command failed with exit code $rc at line $line: $cmd"
  if command -v dialog >/dev/null 2>&1; then
    dialog --title "$TITLE" --msgbox "Command failed with exit code $rc at line $line.\n\nCommand:\n$cmd\n\nLog file:\n$LOG_FILE" 16 78 || true
  fi
  exit "$rc"
}

cleanup_dialog() {
  stty sane >/dev/null 2>&1 || true
  echo ""
  echo "ProIdentity Access VM installer log: $LOG_FILE"
}
trap 'on_error $LINENO' ERR
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
  echo "Installing dialog. Log file: $LOG_FILE"
  log "Installing dialog package"
  apt-get update 2>&1 | tee -a "$LOG_FILE"
  apt-get install -y dialog 2>&1 | tee -a "$LOG_FILE"
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

run_logged() {
  log "RUN: $*"
  "$@" 2>&1 | tee -a "$LOG_FILE"
  local rc="${PIPESTATUS[0]}"
  if [[ "$rc" -ne 0 ]]; then
    return "$rc"
  fi
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
  dialog --title "$TITLE" --msgbox "$1" 14 78 >/dev/tty 2>&1
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
  dialog --title "$TITLE" --yesno "$label" 22 78 >/dev/tty 2>&1
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

# Backward-compatible aliases for early debug builds that had typoed calls.
validate_reired() {
  validate_required "$@"
}

require_collected_value() {
  local name="$1"
  local value="${!name:-}"
  [[ -n "$value" ]] || die "internal error: $name is empty before VM creation. The wizard did not complete correctly, or the script was copied/truncated. Re-run the full script from the beginning."
}

validate_number() {
  local name="$1"
  local value="$2"
  [[ "$value" =~ ^[0-9]+$ ]] || die "$name must be numeric"
}

ip_without_cidr() {
  local value="$1"
  printf "%s" "${value%%/*}"
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

# Backward-compatible alias for early debug builds that had a typoed call.
valite_identifier() {
  validate_identifier "$@"
}

valate_identifier() {
  validate_identifier "$@"
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

run_self_test() {
  bash -n "$0"
  local unknown=""
  unknown="$(
    grep -nE '^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*[[:space:]]+' "$0" |
      sed -n 's/^\([0-9][0-9]*\):[[:space:]]*\([A-Za-z_][A-Za-z0-9_]*\)[[:space:]].*/\1 \2/p' |
      awk '
        BEGIN {
          known["validate_required"]=1
          known["validate_reired"]=1
          known["require_collected_value"]=1
          known["validate_number"]=1
          known["validate_vmid"]=1
          known["validate_identifier"]=1
          known["valite_identifier"]=1
          known["valate_identifier"]=1
          known["validate_linux_user"]=1
          known["validate_secret_value"]=1
          known["validate_hostname"]=1
          known["validate_snippets_storage"]=1
          known["require_cmd"]=1
        }
        $2 ~ /^(val|valid|requ).*_/ && !known[$2] {
          print $1 ":" $2
        }
      '
  )"
  if [[ -n "$unknown" ]]; then
    echo "Unknown validation/helper calls found:" >&2
    echo "$unknown" >&2
    return 1
  fi

  local tmp snippet firstboot
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' RETURN
  snippet="$tmp/proidentity-vm-test-user.yml"
  firstboot="$tmp/proidentity-firstboot.sh"

  VMID="120"
  VM_NAME="proidentity-access"
  HOSTNAME="proidentity-access"
  DB_NAME="proidentity"
  DB_USER="proidentity"
  DB_PASS="testPassword123"
  ADMIN_USER="admin"
  ADMIN_PASS="testAdmin123"
  ADMIN_EMAIL="admin@localhost"
  JWT_SECRET="testJwtSecret1234567890"
  PUBLIC_URL="http://10.10.2.69:8080"
  REPO="$DEFAULT_REPO"
  VERSION="$DEFAULT_VERSION"
  SERVER_HOST="0.0.0.0"
  SERVER_PORT="8080"
  CORS_EXTRA=""
  CI_USER="proidentity"
  CI_PASSWORD="testLogin123"

  write_firstboot_snippet "$snippet"
  awk '
    /^  - path: \/root\/proidentity-firstboot.sh$/ { found = 1 }
    found && /^    content: \|$/ { capture = 1; next }
    capture && /^runcmd:/ { exit }
    capture {
      sub(/^      /, "")
      print
    }
  ' "$snippet" > "$firstboot"
  bash -n "$firstboot"
  grep -q "install_proidentity_release" "$firstboot"
  if grep -q "raw.githubusercontent.com" "$firstboot"; then
    echo "Self-test failed: firstboot still depends on raw main install script." >&2
    return 1
  fi

  echo "Self-test passed: outer script and generated firstboot script syntax look OK."
}

latest_release_version() {
  local latest=""
  latest="$(curl -fsSL "https://api.github.com/repos/${DEFAULT_REPO}/releases/latest" 2>>"$LOG_FILE" |
    sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' |
    head -n 1 || true)"
  if [[ -n "$latest" ]]; then
    printf "%s" "$latest"
  else
    printf "%s" "$DEFAULT_VERSION"
  fi
}

storage_content_list() {
  local storage="$1"
  pvesm config "$storage" 2>/dev/null | awk -F': ' '/^[[:space:]]*content:/ {print $2; exit}'
}

storage_base_path() {
  local storage="$1"
  pvesm config "$storage" 2>/dev/null | awk -F': ' '/^[[:space:]]*path:/ {print $2; exit}'
}

snippet_file_path() {
  local name="$1"
  local path=""
  path="$(pvesm path "${SNIPPET_STORAGE}:snippets/${name}" 2>/dev/null || true)"
  if [[ -n "$path" ]]; then
    printf "%s" "$path"
    return
  fi

  local base
  base="$(storage_base_path "$SNIPPET_STORAGE")"
  [[ -n "$base" ]] || die "selected snippets storage '$SNIPPET_STORAGE' has no filesystem path. Choose a directory-backed storage for cloud-init snippets."
  printf "%s/snippets/%s" "$base" "$name"
}

storage_has_content() {
  local storage="$1"
  local content="$2"
  storage_content_list "$storage" | tr ',' '\n' | grep -qx "$content"
}

enable_snippets_on_storage() {
  local storage="$1"
  local content
  content="$(storage_content_list "$storage")"
  [[ -n "$content" ]] || die "could not read content list for Proxmox storage '$storage'"
  if ! printf "%s" "$content" | tr ',' '\n' | grep -qx snippets; then
    content="${content},snippets"
    log "Enabling snippets content on Proxmox storage $storage"
    pvesm set "$storage" --content "$content" 2>&1 | tee -a "$LOG_FILE"
  fi
}

choose_snippets_storage() {
  local entries=()
  while read -r storage _; do
    [[ -z "$storage" || "$storage" == "Name" ]] && continue
    entries+=("$storage" "Cloud-init snippets storage")
  done < <(pvesm status -content snippets 2>/dev/null | awk 'NR > 1 {print $1, $2}')

  if [[ "${#entries[@]}" -gt 0 ]]; then
    SNIPPET_STORAGE="$(menu "Select cloud-init snippets storage" "${entries[@]}")"
    validate_required "cloud-init snippets storage" "$SNIPPET_STORAGE"
    return
  fi

  if pvesm status "$DEFAULT_SNIPPET_STORAGE" >/dev/null 2>&1; then
    if yesno "No Proxmox storage currently allows snippets content.\n\nCloud-init custom user-data needs snippets.\n\nEnable snippets on '$DEFAULT_SNIPPET_STORAGE' now?"; then
      enable_snippets_on_storage "$DEFAULT_SNIPPET_STORAGE"
      SNIPPET_STORAGE="$DEFAULT_SNIPPET_STORAGE"
      validate_snippets_storage
      return
    fi
  fi

  die "no Proxmox storage allows snippets content. Enable snippets on a storage, for example: pvesm set local --content <existing-content>,snippets"
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

      install_proidentity_release() {
        local target_os="linux"
        local target_arch="amd64"
        local install_dir="/opt/proidentity"
        local config_dir="/etc/proidentity"
        local asset="ProIdentity-Access-Server-\$VERSION-\$target_os-\$target_arch.tar.gz"
        local sums="ProIdentity-Access-\$VERSION-SHA256SUMS.txt"
        local base_url="https://github.com/\$REPO/releases/download/v\$VERSION"
        local tmp

        command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
        command -v sha256sum >/dev/null 2>&1 || { echo "sha256sum is required" >&2; exit 1; }
        command -v tar >/dev/null 2>&1 || { echo "tar is required" >&2; exit 1; }

        tmp="\$(mktemp -d)"
        echo "Downloading \$asset"
        curl -fL "\$base_url/\$asset" -o "\$tmp/\$asset"
        curl -fL "\$base_url/\$sums" -o "\$tmp/\$sums"

        (
          cd "\$tmp"
          tr -d '\r' < "\$sums" > "\$sums.lf"
          if ! awk -v file="\$asset" '
            \$1 ~ /^[0-9a-fA-F]{64}\$/ && (\$2 == file || \$2 == "*" file) {
              print
              found = 1
            }
            END { exit found ? 0 : 1 }
          ' "\$sums.lf" > "\$asset.sha256"; then
            echo "ERROR: checksum file does not contain an entry for \$asset" >&2
            echo "Downloaded checksum file contents:" >&2
            cat "\$sums.lf" >&2
            exit 1
          fi
          sha256sum -c "\$asset.sha256"
        )

        mkdir -p "\$tmp/package"
        tar -xzf "\$tmp/\$asset" -C "\$tmp/package"

        install -d -m 0755 "\$install_dir/bin" "\$install_dir/migrations" "\$config_dir"
        install -m 0755 "\$tmp/package/bin/proidentity" "\$install_dir/bin/proidentity"

        rm -rf "\$install_dir/migrations"
        install -d -m 0755 "\$install_dir/migrations"
        cp -R "\$tmp/package/migrations/." "\$install_dir/migrations/"

        if [ ! -f "\$config_dir/config.yaml" ]; then
          install -m 0640 "\$tmp/package/config.example.yaml" "\$config_dir/config.yaml"
        fi

        install -m 0644 "\$tmp/package/systemd/proidentity.service" /etc/systemd/system/proidentity.service
        systemctl daemon-reload
        systemctl enable proidentity >/dev/null
        rm -rf "\$tmp"
      }

      install_proidentity_release

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
  DOWNLOADED_IMAGE_PATH="$IMAGE_DIR/proidentity-debian-13-generic-amd64.qcow2"
  if [[ ! -s "$DOWNLOADED_IMAGE_PATH" ]]; then
    msg "The Debian 13 cloud image will be downloaded now. This can take a few minutes."
    curl -fL "$IMAGE_URL" -o "$DOWNLOADED_IMAGE_PATH" 2>&1 | tee -a "$LOG_FILE"
  fi
}

create_vm() {
  local image_path="$1"
  require_collected_value VMID
  require_collected_value VM_NAME
  require_collected_value STORAGE
  require_collected_value BRIDGE
  require_collected_value DISK_GB
  require_collected_value CORES
  require_collected_value MEMORY_MB
  require_collected_value DNS_SERVER
  require_collected_value IP_MODE
  require_collected_value SNIPPET_STORAGE
  validate_number "VM ID" "$VMID"
  validate_number "disk size" "$DISK_GB"
  validate_number "CPU cores" "$CORES"
  validate_number "memory" "$MEMORY_MB"
  if [[ "$IP_MODE" == "static" ]]; then
    require_collected_value IP_CIDR
    require_collected_value GATEWAY
  fi

  local net0="virtio,bridge=${BRIDGE}"
  if [[ -n "$VLAN_TAG" ]]; then
    net0="${net0},tag=${VLAN_TAG}"
  fi

  log "Creating VM $VMID name=$VM_NAME storage=$STORAGE bridge=$BRIDGE ip_mode=$IP_MODE"
  qm create "$VMID" \
    --name "$VM_NAME" \
    --ostype l26 \
    --memory "$MEMORY_MB" \
    --cores "$CORES" \
    --cpu host \
    --agent enabled=1 \
    --scsihw virtio-scsi-pci \
    --net0 "$net0" \
    --vga std

  log "Importing cloud image $image_path into storage $STORAGE"
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

  log "Resizing VM $VMID disk to ${DISK_GB}G"
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
  choose_snippets_storage
  BRIDGE="$(choose_bridge)"
  VLAN_TAG="$(input "VLAN tag. Leave empty for none." "")"
  DISK_GB="$(input "Disk size in GB" "32")"
  CORES="$(input "CPU cores" "2")"
  MEMORY_MB="$(input "Memory in MB" "4096")"
  validate_number "disk size" "$DISK_GB"
  validate_number "CPU cores" "$CORES"
  validate_number "memory" "$MEMORY_MB"

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
  validate_required "DNS server" "$DNS_SERVER"

  REPO="$DEFAULT_REPO"
  VERSION="$(latest_release_version)"
  msg "Using public GitHub repository:\nhttps://github.com/${REPO}\n\nLatest release detected:\nv${VERSION}\n\nNo GitHub username, password, or SSH key is needed."
  SERVER_PORT="$(input "ProIdentity HTTP port" "8080")"
  validate_number "ProIdentity HTTP port" "$SERVER_PORT"
  VERSION="$(input "ProIdentity release version" "$VERSION")"
  IMAGE_URL="$(input "Debian 13 cloud image URL" "$DEFAULT_IMAGE_URL")"

  LISTEN_MODE="$(menu "HTTP listen mode" \
    "direct" "No Nginx: listen on 0.0.0.0" \
    "proxy" "Reverse proxy: listen on 127.0.0.1")"
  if [[ "$LISTEN_MODE" == "direct" ]]; then
    SERVER_HOST="0.0.0.0"
  else
    SERVER_HOST="127.0.0.1"
  fi
  if [[ "$IP_MODE" == "static" ]]; then
    DEFAULT_PUBLIC_URL="http://$(ip_without_cidr "$IP_CIDR"):${SERVER_PORT}"
  else
    DEFAULT_PUBLIC_URL="http://<vm-ip>:${SERVER_PORT}"
  fi
  PUBLIC_URL="$(input "Public or direct server URL. For direct mode this was generated from the VM IP you entered." "$DEFAULT_PUBLIC_URL")"
  CORS_EXTRA="$(input "Extra CORS origin. Leave empty if not needed." "")"

  DB_NAME="$(input "MariaDB database name. Leave empty for proidentity." "proidentity")"
  DB_USER="$(input "MariaDB app user. Leave empty to generate." "proidentity")"
  DB_PASS="$(password_input "MariaDB app password. Leave empty to generate." "")"
  JWT_SECRET="$(password_input "JWT secret. Leave empty to generate." "")"
  ADMIN_USER="$(input "Initial admin username" "admin")"
  ADMIN_EMAIL="$(input "Initial admin email" "admin@localhost")"
  ADMIN_PASS="$(password_input "Initial admin password. Leave empty to generate." "")"

  CI_USER="$(input "VM login user" "proidentity")"
  CI_PASSWORD="$(password_input "VM login password. Leave empty to generate." "")"

  if [[ -z "$DB_NAME" ]]; then DB_NAME="proidentity"; fi
  if [[ -z "$DB_USER" ]]; then DB_USER="proidentity_$(random_alnum 8 | tr 'A-Z' 'a-z')"; fi
  if [[ -z "$DB_PASS" ]]; then DB_PASS="$(random_alnum 32)"; fi
  if [[ -z "$JWT_SECRET" ]]; then JWT_SECRET="$(random_alnum 64)"; fi
  if [[ -z "$ADMIN_PASS" ]]; then ADMIN_PASS="$(random_alnum 24)"; fi
  if [[ -z "$CI_PASSWORD" ]]; then CI_PASSWORD="$(random_alnum 24)"; fi

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
  validate_required "VM login user" "$CI_USER"
  validate_required "VM login password" "$CI_PASSWORD"
  validate_secret_value "Database password" "$DB_PASS"
  validate_secret_value "JWT secret" "$JWT_SECRET"
  validate_identifier "Admin username" "$ADMIN_USER"
  validate_secret_value "Admin password" "$ADMIN_PASS"
  validate_linux_user "VM login user" "$CI_USER"
  validate_secret_value "VM login password" "$CI_PASSWORD"
  validate_required "VM storage" "$STORAGE"
  validate_required "network bridge" "$BRIDGE"

  local summary
  summary="VM ID: $VMID
VM name: $VM_NAME
Storage: $STORAGE
Bridge: $BRIDGE
Network: $IP_MODE ${IP_CIDR:-}
Disk: ${DISK_GB}G
CPU/RAM: ${CORES} cores / ${MEMORY_MB} MB
Release: https://github.com/$REPO/releases/tag/v$VERSION
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
  require_cmd curl
  require_cmd ip
  require_cmd openssl

  collect_inputs

  require_collected_value VMID
  require_collected_value SNIPPET_STORAGE
  local snippet_name="proidentity-vm-${VMID}-user.yml"
  local snippet
  snippet="$(snippet_file_path "$snippet_name")"
  mkdir -p "$(dirname "$snippet")"
  log "Writing cloud-init snippet to $snippet for storage ${SNIPPET_STORAGE}:snippets/${snippet_name}"
  write_firstboot_snippet "$snippet"
  [[ -s "$snippet" ]] || die "failed to write cloud-init snippet: $snippet"

  DOWNLOADED_IMAGE_PATH=""
  download_image
  require_collected_value DOWNLOADED_IMAGE_PATH
  create_vm "$DOWNLOADED_IMAGE_PATH"

  qm set "$VMID" --description "ProIdentity Access server. First boot log: /root/proidentity-firstboot.log" >/dev/null
  log "Starting VM $VMID"
  if ! run_logged qm start "$VMID"; then
    log "VM $VMID failed to start. Dumping config and status."
    {
      echo "--- qm status $VMID ---"
      qm status "$VMID" || true
      echo "--- qm config $VMID ---"
      qm config "$VMID" || true
      echo "--- recent tasks mentioning VM $VMID ---"
      pvesh get /cluster/tasks --output-format json 2>/dev/null | grep -E "\"id\":\"$VMID\"|\"upid\"|\"status\"|\"type\"" | head -n 80 || true
    } 2>&1 | tee -a "$LOG_FILE"
    die "failed to start VM $VMID. Check Proxmox task log and installer log: $LOG_FILE"
  fi

  msg "VM $VMID has been created and started.\n\nFirst boot will install MariaDB, WireGuard tools, ProIdentity Access $VERSION, write all config/env values, and start the service.\n\nGuest setup log:\n/root/proidentity-firstboot.log\n\nSummary file:\n/root/proidentity-install-summary.txt\n\nInitial admin: $ADMIN_USER\nInitial admin password: $ADMIN_PASS\n\nSSH user: $CI_USER\nSSH password: $CI_PASSWORD"
}

if [[ "${1:-}" == "--self-test" ]]; then
  run_self_test
  exit $?
fi

main "$@"
