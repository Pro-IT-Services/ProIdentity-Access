#!/usr/bin/env sh
set -eu

usage() {
  cat >&2 <<'EOF'
Usage: install-release.sh OWNER/REPO VERSION

Example:
  sudo ./server/install-release.sh Pro-IT-Services/ProIdentity-Access 0.5.20

Environment:
  PROIDENTITY_INSTALL_DIR   default: /opt/proidentity
  PROIDENTITY_CONFIG_DIR    default: /etc/proidentity
  PROIDENTITY_SERVICE_NAME  default: proidentity
  PROIDENTITY_SERVER_OS     default: linux
  PROIDENTITY_SERVER_ARCH   default: amd64
  PROIDENTITY_DATABASE_NAME default: proidentity
  PROIDENTITY_DATABASE_USER generated when not set
  PROIDENTITY_DATABASE_PASS generated when not set
  PROIDENTITY_DATABASE_HOST default: 127.0.0.1:3306
  PROIDENTITY_DATABASE_DSN  generated when not set
EOF
}

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR: run as root." >&2
  exit 1
fi

if [ "$#" -lt 2 ]; then
  usage
  exit 1
fi

repo="$1"
version="$2"
target_os="${PROIDENTITY_SERVER_OS:-linux}"
target_arch="${PROIDENTITY_SERVER_ARCH:-amd64}"
install_dir="${PROIDENTITY_INSTALL_DIR:-/opt/proidentity}"
config_dir="${PROIDENTITY_CONFIG_DIR:-/etc/proidentity}"
service_name="${PROIDENTITY_SERVICE_NAME:-proidentity}"

asset="ProIdentity-Access-Server-$version-$target_os-$target_arch.tar.gz"
sums="ProIdentity-Access-$version-SHA256SUMS.txt"
base_url="https://github.com/$repo/releases/download/v$version"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: $1 is required." >&2
    exit 1
  fi
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -base64 48 | tr -d '\n'
  else
    dd if=/dev/urandom bs=48 count=1 2>/dev/null | base64 | tr -d '\n'
  fi
}

random_alnum() {
  length="${1:-32}"
  LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | dd bs=1 count="$length" 2>/dev/null
}

sql_escape() {
  printf "%s" "$1" | sed "s/'/''/g"
}

require_cmd curl
require_cmd sha256sum
require_cmd tar
require_cmd systemctl

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $asset"
curl -fL "$base_url/$asset" -o "$tmp/$asset"
curl -fL "$base_url/$sums" -o "$tmp/$sums"

(
  cd "$tmp"
  tr -d '\r' < "$sums" > "$sums.lf"
  if ! awk -v file="$asset" '
    $1 ~ /^[0-9a-fA-F]{64}$/ && ($2 == file || $2 == "*" file) {
      print
      found = 1
    }
    END { exit found ? 0 : 1 }
  ' "$sums.lf" > "$asset.sha256"; then
    echo "ERROR: checksum file does not contain an entry for $asset" >&2
    echo "Downloaded checksum file contents:" >&2
    cat "$sums.lf" >&2
    exit 1
  fi
  sha256sum -c "$asset.sha256"
)

mkdir -p "$tmp/package"
tar -xzf "$tmp/$asset" -C "$tmp/package"

install -d -m 0755 "$install_dir/bin" "$install_dir/migrations" "$config_dir"
install -m 0755 "$tmp/package/bin/proidentity" "$install_dir/bin/proidentity"

rm -rf "$install_dir/migrations"
install -d -m 0755 "$install_dir/migrations"
cp -R "$tmp/package/migrations/." "$install_dir/migrations/"

if [ ! -f "$config_dir/config.yaml" ]; then
  install -m 0640 "$tmp/package/config.example.yaml" "$config_dir/config.yaml"
  echo "Wrote $config_dir/config.yaml"
fi

if [ ! -f "$config_dir/proidentity.env" ]; then
  jwt_secret="$(random_secret)"
  admin_pass="$(random_secret | cut -c 1-24)"
  db_name="${PROIDENTITY_DATABASE_NAME:-proidentity}"
  db_user="${PROIDENTITY_DATABASE_USER:-proidentity_$(random_alnum 8)}"
  db_pass="${PROIDENTITY_DATABASE_PASS:-$(random_alnum 32)}"
  db_host="${PROIDENTITY_DATABASE_HOST:-127.0.0.1:3306}"
  db_dsn="${PROIDENTITY_DATABASE_DSN:-$db_user:$db_pass@tcp($db_host)/$db_name}"
  case "$db_name" in
    ""|*[!A-Za-z0-9_]*)
      echo "ERROR: PROIDENTITY_DATABASE_NAME may contain only letters, numbers, and underscores." >&2
      exit 1
      ;;
  esac
  case "$db_user" in
    ""|*[!A-Za-z0-9_]*)
      echo "ERROR: PROIDENTITY_DATABASE_USER may contain only letters, numbers, and underscores." >&2
      exit 1
      ;;
  esac
  case "$db_pass" in
    ""|*[!A-Za-z0-9_.-]*)
      echo "ERROR: PROIDENTITY_DATABASE_PASS may contain only letters, numbers, dot, underscore, and dash." >&2
      exit 1
      ;;
  esac
  cat > "$config_dir/proidentity.env" <<EOF
PROIDENTITY_JWT_SECRET=$jwt_secret
PROIDENTITY_DATABASE_DSN=$db_dsn
WG_ADMIN_USER=admin
WG_ADMIN_PASS=$admin_pass
WG_ADMIN_EMAIL=admin@localhost
EOF
  chmod 0600 "$config_dir/proidentity.env"
  echo "Wrote $config_dir/proidentity.env"
  echo "Initial admin password: $admin_pass"
  echo "Generated database name: $db_name"
  echo "Generated database user: $db_user"
  echo "Generated database password: $db_pass"
  echo ""
  echo "If the database/user do not already exist, create them with:"
  cat <<SQL
CREATE DATABASE IF NOT EXISTS \`$(sql_escape "$db_name")\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '$(sql_escape "$db_user")'@'localhost' IDENTIFIED BY '$(sql_escape "$db_pass")';
ALTER USER '$(sql_escape "$db_user")'@'localhost' IDENTIFIED BY '$(sql_escape "$db_pass")';
GRANT ALL PRIVILEGES ON \`$(sql_escape "$db_name")\`.* TO '$(sql_escape "$db_user")'@'localhost';
FLUSH PRIVILEGES;
SQL
fi

install -m 0644 "$tmp/package/systemd/proidentity.service" "/etc/systemd/system/$service_name.service"
systemctl daemon-reload
systemctl enable "$service_name.service" >/dev/null

cat <<EOF

Install complete.

Before starting the service:
  1. Edit $config_dir/config.yaml and set the database DSN.
  2. Create the MariaDB database and user.
  3. Configure your reverse proxy to http://127.0.0.1:8080.

Start when ready:
  systemctl start $service_name
  systemctl status $service_name --no-pager

EOF
