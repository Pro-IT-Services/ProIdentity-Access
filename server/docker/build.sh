#!/usr/bin/env sh
set -eu
. "$(dirname "$0")/common.sh"

ensure_env
cd "$PROJECT_DIR"
compose build proidentity

