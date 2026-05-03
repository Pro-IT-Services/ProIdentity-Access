#!/usr/bin/env sh
set -eu
. "$(dirname "$0")/common.sh"

ensure_env
compose up -d --build
compose ps

