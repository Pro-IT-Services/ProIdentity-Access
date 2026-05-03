#!/bin/bash
# build.sh — Full macOS build: Frontend → App → Daemon → .pkg installer
#
# Usage:
#   ./build.sh                  # full build
#   ./build.sh --skip-ui        # skip npm install/build (reuse existing frontend/dist)
#   ./build.sh --version 1.2.3  # override version
#
# Prerequisites:
#   go     1.22+   https://go.dev/dl
#   node   18+     https://nodejs.org
#   wails  v2      go install github.com/wailsapp/wails/v2/cmd/wails@latest
#   Xcode Command Line Tools (pkgbuild)  xcode-select --install

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="$REPO_ROOT/build/bin"
export PATH="$PATH:$(go env GOPATH)/bin"

# ── Args ──────────────────────────────────────────────────────────────────────
SKIP_UI=false
VERSION=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --skip-ui)   SKIP_UI=true; shift ;;
        --version)   VERSION="$2"; shift 2 ;;
        *)           echo "Unknown argument: $1" >&2; exit 1 ;;
    esac
done

if [[ -z "$VERSION" ]]; then
    VERSION=$(python3 -c "import json,sys; print(json.load(open('$REPO_ROOT/wails.json'))['info']['productVersion'])" 2>/dev/null \
              || node -p "require('$REPO_ROOT/wails.json').info.productVersion" 2>/dev/null \
              || echo "0.1.0")
fi

echo ""
echo "  ProIdentity Access  v$VERSION  (macOS)"
echo ""

# ── Prerequisites ─────────────────────────────────────────────────────────────
MISSING=false
check() {
    if ! command -v "$1" &>/dev/null; then
        echo "  [MISSING] $1  ->  $2"
        MISSING=true
    fi
}
check go    "https://go.dev/dl"
check node  "https://nodejs.org"
check npm   "https://nodejs.org"
check wails "go install github.com/wailsapp/wails/v2/cmd/wails@latest"
check pkgbuild "xcode-select --install"
$MISSING && exit 1

# ── Step 1 — Frontend ─────────────────────────────────────────────────────────
if $SKIP_UI; then
    echo "[1/4] Frontend — skipped (--skip-ui)"
else
    echo "[1/4] Frontend — npm install"
    (cd "$REPO_ROOT/frontend" && npm install --prefer-offline --no-audit --no-fund)

    echo "[1/4] Frontend — npm run build"
    (cd "$REPO_ROOT/frontend" && npm run build)
fi

# ── Step 2 — GUI app (Wails embeds frontend into the binary) ──────────────────
# Sync app icon from assets (Wails converts this to .icns for the app bundle)
mkdir -p "$REPO_ROOT/build"
cp "$REPO_ROOT/assets/trayicon.png" "$REPO_ROOT/build/appicon.png"

echo "[2/4] App — wails build (darwin/$(uname -m))"
cd "$REPO_ROOT"
# -s skips the frontend build inside wails; step 1 already handled it
rm -rf "$BIN_DIR/ProIdentity.app" "$BIN_DIR/ProIdentity Access.app"
wails build -s
[ -d "$BIN_DIR/ProIdentity Access.app" ] || { echo "ERROR: ProIdentity Access.app not found" >&2; exit 1; }

# ── Step 3 — Daemon ───────────────────────────────────────────────────────────
echo "[3/4] Daemon — go build"
cd "$REPO_ROOT"
go build -ldflags="-s -w" -o "$BIN_DIR/proidentity-daemon" ./cmd/daemon/
[ -f "$BIN_DIR/proidentity-daemon" ] || { echo "ERROR: proidentity-daemon not found" >&2; exit 1; }

# ── Step 4 — .pkg installer ───────────────────────────────────────────────────
echo "[4/4] Installer — pkgbuild"
SCRIPTS_DIR="$REPO_ROOT/installer/darwin/scripts"
PKG_ROOT="$(mktemp -d)"
trap 'rm -rf "$PKG_ROOT"' EXIT
OUTPUT_PKG="$REPO_ROOT/build/darwin/ProIdentity-Access-$VERSION.pkg"
mkdir -p "$PKG_ROOT/Applications"
mkdir -p "$PKG_ROOT/Library/ProIdentity"
mkdir -p "$PKG_ROOT/Library/LaunchAgents"

cp -R "$BIN_DIR/ProIdentity Access.app" "$PKG_ROOT/Applications/"
cp    "$BIN_DIR/proidentity-daemon"    "$PKG_ROOT/Library/ProIdentity/proidentity-daemon"
cp    "$REPO_ROOT/installer/darwin/launch_agent/com.proitservices.proidentity.access-ui.plist" \
      "$PKG_ROOT/Library/LaunchAgents/com.proitservices.proidentity.access-ui.plist"

# Bundle the uninstaller app
cp -R "$REPO_ROOT/installer/darwin/uninstaller_app" \
      "$PKG_ROOT/Applications/ProIdentity Access Uninstaller.app"
chmod +x "$PKG_ROOT/Applications/ProIdentity Access Uninstaller.app/Contents/MacOS/ProIdentity Uninstaller"

pkgbuild \
    --root             "$PKG_ROOT" \
    --scripts          "$SCRIPTS_DIR" \
    --identifier       "com.proitservices.proidentity.access.installer" \
    --version          "$VERSION" \
    --ownership        recommended \
    --install-location "/" \
    "$OUTPUT_PKG"

# ── Done ──────────────────────────────────────────────────────────────────────
SIZE=$(du -sh "$OUTPUT_PKG" | cut -f1)
echo ""
echo "  Build complete"
echo "  $OUTPUT_PKG  ($SIZE)"
echo ""
echo "  Install:  sudo installer -pkg \"$OUTPUT_PKG\" -target /"
echo "  Or double-click the .pkg in Finder."
echo ""
