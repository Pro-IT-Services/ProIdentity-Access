#!/usr/bin/env sh
set -eu

if [ -z "${PROIDENTITY_JWT_SECRET:-}" ]; then
  echo "ERROR: PROIDENTITY_JWT_SECRET must be set." >&2
  exit 1
fi

if [ -z "${WG_ADMIN_PASS:-}" ] && [ "${PROIDENTITY_ALLOW_INSECURE_DEFAULTS:-}" != "1" ]; then
  echo "ERROR: WG_ADMIN_PASS must be set before first boot." >&2
  exit 1
fi

if [ ! -c /dev/net/tun ]; then
  echo "ERROR: /dev/net/tun is missing. Add devices: /dev/net/tun:/dev/net/tun." >&2
  exit 1
fi

if [ -n "${PROIDENTITY_DATABASE_DSN:-}" ]; then
  db_addr="$(printf '%s' "$PROIDENTITY_DATABASE_DSN" | sed -n 's/.*@tcp(\([^)]*\)).*/\1/p')"
  if [ -n "$db_addr" ]; then
    db_host="${db_addr%:*}"
    db_port="${db_addr##*:}"
    if [ "$db_host" = "$db_port" ]; then
      db_port="3306"
    fi
    i=0
    while ! nc -z "$db_host" "$db_port" >/dev/null 2>&1; do
      i=$((i + 1))
      if [ "$i" -ge 60 ]; then
        echo "ERROR: database not reachable at $db_host:$db_port." >&2
        exit 1
      fi
      sleep 2
    done
  fi
fi

if [ "$#" -eq 0 ]; then
  set -- /app/config.yaml
fi

exec /app/proidentity "$@"

