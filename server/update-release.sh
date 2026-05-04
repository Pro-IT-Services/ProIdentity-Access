#!/usr/bin/env sh
set -eu

usage() {
  cat >&2 <<'EOF'
Usage: update-release.sh [OWNER/REPO]

Checks the installed ProIdentity Access server version against the latest
GitHub release and installs the server package when a newer version exists.

Environment:
  PROIDENTITY_REPO          default: Pro-IT-Services/ProIdentity-Access
  PROIDENTITY_INSTALL_DIR   default: /opt/proidentity
  PROIDENTITY_CONFIG_DIR    default: /etc/proidentity
  PROIDENTITY_SERVICE_NAME  default: proidentity
  PROIDENTITY_SERVER_OS     default: linux
  PROIDENTITY_SERVER_ARCH   default: amd64
  PROIDENTITY_FORCE_UPDATE  set to 1 to reinstall even when not newer
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR: run as root." >&2
  exit 1
fi

repo="${1:-${PROIDENTITY_REPO:-Pro-IT-Services/ProIdentity-Access}}"
target_os="${PROIDENTITY_SERVER_OS:-linux}"
target_arch="${PROIDENTITY_SERVER_ARCH:-amd64}"
install_dir="${PROIDENTITY_INSTALL_DIR:-/opt/proidentity}"
config_dir="${PROIDENTITY_CONFIG_DIR:-/etc/proidentity}"
service_name="${PROIDENTITY_SERVICE_NAME:-proidentity}"
version_file="$install_dir/VERSION"
force="${PROIDENTITY_FORCE_UPDATE:-0}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: $1 is required." >&2
    exit 1
  fi
}

version_gt() {
  current="$1"
  latest="$2"
  [ "$current" != "$latest" ] || return 1
  newest="$(printf '%s\n%s\n' "$current" "$latest" | sort -V | tail -n 1)"
  [ "$newest" = "$latest" ]
}

release_json_value() {
  key="$1"
  sed -n "s/.*\"$key\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -n 1
}

require_cmd curl
require_cmd sha256sum
require_cmd sort
require_cmd tar
require_cmd systemctl

latest_tag="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | release_json_value tag_name)"
if [ -z "$latest_tag" ]; then
  echo "ERROR: could not determine latest release for $repo." >&2
  exit 1
fi
latest_version="${latest_tag#v}"

local_version="unknown"
if [ -f "$version_file" ]; then
  local_version="$(tr -d ' \t\r\n' < "$version_file")"
fi

echo "Installed version: $local_version"
echo "Latest release:    $latest_version"

if [ "$force" != "1" ] && [ "$local_version" != "unknown" ] && ! version_gt "$local_version" "$latest_version"; then
  echo "Already up to date."
  exit 0
fi

if [ "$local_version" = "unknown" ]; then
  echo "Local version is unknown; installing latest release and writing $version_file."
fi

asset="ProIdentity-Access-Server-$latest_version-$target_os-$target_arch.tar.gz"
sums="ProIdentity-Access-$latest_version-SHA256SUMS.txt"
base_url="https://github.com/$repo/releases/download/v$latest_version"

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
    cat "$sums.lf" >&2
    exit 1
  fi
  sha256sum -c "$asset.sha256"
)

mkdir -p "$tmp/package"
tar -xzf "$tmp/$asset" -C "$tmp/package"

backup_dir="$install_dir/backups/pre-update-$(date -u +%Y%m%d%H%M%S)"
mkdir -p "$backup_dir"
if [ -f "$install_dir/bin/proidentity" ]; then
  cp "$install_dir/bin/proidentity" "$backup_dir/proidentity"
fi
if [ -d "$install_dir/migrations" ]; then
  cp -R "$install_dir/migrations" "$backup_dir/migrations"
fi
if [ -f "$version_file" ]; then
  cp "$version_file" "$backup_dir/VERSION"
fi

was_active="0"
if systemctl is-active --quiet "$service_name.service"; then
  was_active="1"
  systemctl stop "$service_name.service"
fi

install -d -m 0755 "$install_dir/bin" "$install_dir/migrations" "$config_dir"
install -m 0755 "$tmp/package/bin/proidentity" "$install_dir/bin/proidentity"

rm -rf "$install_dir/migrations"
install -d -m 0755 "$install_dir/migrations"
cp -R "$tmp/package/migrations/." "$install_dir/migrations/"

if [ -f "$tmp/package/systemd/proidentity.service" ]; then
  install -m 0644 "$tmp/package/systemd/proidentity.service" "/etc/systemd/system/$service_name.service"
fi
if [ -f "$tmp/package/install-release.sh" ]; then
  install -m 0755 "$tmp/package/install-release.sh" "$install_dir/install-release.sh"
fi
if [ -f "$tmp/package/update-release.sh" ]; then
  install -m 0755 "$tmp/package/update-release.sh" "$install_dir/update-release.sh"
fi

printf "%s\n" "$latest_version" > "$version_file"
systemctl daemon-reload
systemctl enable "$service_name.service" >/dev/null

if [ "$was_active" = "1" ]; then
  systemctl start "$service_name.service"
  systemctl is-active --quiet "$service_name.service"
fi

echo "Updated ProIdentity Access server to $latest_version."
echo "Backup: $backup_dir"
