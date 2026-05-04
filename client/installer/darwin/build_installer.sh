#!/bin/bash
# build_installer.sh - builds ProIdentity Access for macOS
# Usage: ./installer/darwin/build_installer.sh [--skip-build]
#
# Output: build/darwin/ProIdentity-Access-<version>.pkg
#
# Requirements: Xcode Command Line Tools  (pkgbuild is part of them)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPTS_DIR="$(dirname "$0")/scripts"
BUILD_DIR="$REPO_ROOT/build"
PKG_ROOT="$BUILD_DIR/darwin/pkg_root"          # assembled install tree (gitignored)
VERSION=$(python3 -c "import json; print(json.load(open('$REPO_ROOT/wails.json'))['info']['productVersion'])" 2>/dev/null \
          || node -p "require('$REPO_ROOT/wails.json').info.productVersion" 2>/dev/null \
          || echo "0.5.25")
OUTPUT_PKG="$BUILD_DIR/darwin/ProIdentity-Access-$VERSION.pkg"

APP_SRC="$BUILD_DIR/bin/ProIdentity Access.app"
DAEMON_SRC="$BUILD_DIR/bin/proidentity-daemon"

# â”€â”€ 1. Build binaries â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
if [[ "${1:-}" != "--skip-build" ]]; then
    echo "==> Building ProIdentity Access.app â€¦"
    cd "$REPO_ROOT"
    export PATH="$PATH:$(go env GOPATH)/bin"
    wails build

    echo "==> Building daemon â€¦"
    go build -o "$DAEMON_SRC" ./cmd/daemon/
fi

# â”€â”€ 2. Verify outputs â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
[ -d "$APP_SRC" ]  || { echo "ERROR: ProIdentity Access.app not found" >&2; exit 1; }
[ -f "$DAEMON_SRC" ] || { echo "ERROR: proidentity-daemon not found" >&2; exit 1; }

# â”€â”€ 3. Assemble package root â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
echo "==> Assembling package root â€¦"
# Use chmod first to handle root-owned leftovers from a previous pkg install
chmod -R u+w "$PKG_ROOT" 2>/dev/null || true
rm -rf "$PKG_ROOT"
mkdir -p "$PKG_ROOT/Applications"
mkdir -p "$PKG_ROOT/Library/ProIdentity"
mkdir -p "$PKG_ROOT/Library/LaunchAgents"

cp -R "$APP_SRC"   "$PKG_ROOT/Applications/"
cp    "$DAEMON_SRC" "$PKG_ROOT/Library/ProIdentity/proidentity-daemon"
cp    "$REPO_ROOT/LICENSE" "$PKG_ROOT/Library/ProIdentity/LICENSE"
cp    "$REPO_ROOT/installer/darwin/launch_agent/com.proitservices.proidentity.access-ui.plist" \
      "$PKG_ROOT/Library/LaunchAgents/com.proitservices.proidentity.access-ui.plist"

# Bundle the uninstaller app
cp -R "$REPO_ROOT/installer/darwin/uninstaller_app" \
      "$PKG_ROOT/Applications/ProIdentity Access Uninstaller.app"
chmod +x "$PKG_ROOT/Applications/ProIdentity Access Uninstaller.app/Contents/MacOS/ProIdentity Uninstaller"

# â”€â”€ 4. Build .pkg â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
echo "==> Running pkgbuild â€¦"
pkgbuild \
    --root             "$PKG_ROOT" \
    --scripts          "$SCRIPTS_DIR" \
    --identifier       "com.proitservices.proidentity.access.installer" \
    --version          "$VERSION" \
    --ownership        recommended \
    --install-location "/" \
    "$OUTPUT_PKG"

echo ""
echo "âś“  $OUTPUT_PKG"
echo ""
echo "Install:  sudo installer -pkg \"$OUTPUT_PKG\" -target /"
echo "Or double-click the .pkg file in Finder."
