#!/usr/bin/env sh
set -eu
. "$(dirname "$0")/common.sh"

ensure_env
backup_dir="$SCRIPT_DIR/backups"
mkdir -p "$backup_dir"
stamp="$(date +%Y%m%d-%H%M%S)"
out="$backup_dir/proidentity-db-$stamp.sql.gz"
compose exec -T db sh -c 'mariadb-dump -uroot -p"$MARIADB_ROOT_PASSWORD" "$MARIADB_DATABASE"' | gzip > "$out"
echo "$out"

