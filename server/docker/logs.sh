#!/usr/bin/env sh
set -eu
. "$(dirname "$0")/common.sh"

service="${1:-proidentity}"
compose logs -f --tail=200 "$service"

