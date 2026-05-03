#!/bin/bash
# uninstall.sh - removes ProIdentity Access from macOS
# Usage: sudo ./installer/darwin/uninstall.sh

set -euo pipefail

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root:  sudo $0"
    exit 1
fi

DAEMON_PLIST=/Library/LaunchDaemons/com.proitservices.proidentity.access.daemon.plist
OLD_DAEMON_PLIST=/Library/LaunchDaemons/com.proidentity.app.plist
AGENT_PLIST=/Library/LaunchAgents/com.proitservices.proidentity.access-ui.plist
OLD_AGENT_PLIST=/Library/LaunchAgents/com.proidentity.app-ui.plist
DAEMON_DIR=/Library/ProIdentity
APP="/Applications/ProIdentity Access.app"
OLD_APP=/Applications/ProIdentity.app
DATA_DIR="/Library/Application Support/ProIdentity"
LOG_DIR=/Library/Logs/ProIdentity

echo "==> Stopping daemon"
for plist in "$DAEMON_PLIST" "$OLD_DAEMON_PLIST"; do
    launchctl bootout system "$plist" 2>/dev/null || \
    launchctl unload "$plist" 2>/dev/null || true
done

echo "==> Stopping UI launch agent"
CURRENT_USER=$(stat -f "%Su" /dev/console 2>/dev/null || true)
if [ -n "$CURRENT_USER" ] && [ "$CURRENT_USER" != "root" ]; then
    USER_UID=$(id -u "$CURRENT_USER")
    for plist in "$AGENT_PLIST" "$OLD_AGENT_PLIST"; do
        launchctl bootout gui/"$USER_UID" "$plist" 2>/dev/null || true
    done
fi

echo "==> Removing launchd files"
rm -f "$DAEMON_PLIST" "$OLD_DAEMON_PLIST" "$AGENT_PLIST" "$OLD_AGENT_PLIST"

echo "==> Removing daemon"
rm -rf "$DAEMON_DIR"

echo "==> Removing app"
rm -rf "$APP" "$OLD_APP" "/Applications/ProIdentity Access Uninstaller.app" "/Applications/ProIdentity Uninstaller.app"

echo "==> Removing logs"
rm -rf "$LOG_DIR"

if [ -d "$DATA_DIR" ]; then
    read -r -p "Remove tunnel config data ($DATA_DIR)? [y/N] " answer
    if [[ "$answer" =~ ^[Yy]$ ]]; then
        rm -rf "$DATA_DIR"
        echo "    Removed $DATA_DIR"
    else
        echo "    Kept $DATA_DIR"
    fi
fi

echo ""
echo "ProIdentity Access uninstalled."
