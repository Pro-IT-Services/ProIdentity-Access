#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
PROJECT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$SCRIPT_DIR/.env"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"

compose() {
  docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" "$@"
}

random_secret() {
  length="${1:-48}"
  tr -dc 'A-Za-z0-9' < /dev/urandom | head -c "$length"
}

ensure_env() {
  if [ ! -f "$ENV_FILE" ]; then
    cp "$SCRIPT_DIR/.env.example" "$ENV_FILE"
    db_pass="$(random_secret 32)"
    root_pass="$(random_secret 32)"
    jwt_secret="$(random_secret 64)"
    admin_pass="$(random_secret 24)"
    sed -i "s/replace-with-random-db-password/$db_pass/" "$ENV_FILE"
    sed -i "s/replace-with-random-root-password/$root_pass/" "$ENV_FILE"
    sed -i "s/replace-with-random-secret-at-least-32-characters/$jwt_secret/" "$ENV_FILE"
    sed -i "s/replace-with-random-admin-password/$admin_pass/" "$ENV_FILE"
    chmod 0600 "$ENV_FILE"
    echo "Wrote $ENV_FILE"
    echo "Initial admin password: $admin_pass"
  fi
}

