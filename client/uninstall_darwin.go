//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// UninstallApp runs the full uninstall sequence on macOS.
// It writes a temp shell script and runs it via osascript so macOS shows
// its native admin-password dialog. The app quits on success.
func (a *App) UninstallApp(keepData bool) error {
	script := `#!/bin/bash
# Stop and remove daemon
launchctl bootout system /Library/LaunchDaemons/com.proitservices.proidentity.access.daemon.plist 2>/dev/null
launchctl bootout system /Library/LaunchDaemons/com.proidentity.app.plist 2>/dev/null
launchctl unload /Library/LaunchDaemons/com.proitservices.proidentity.access.daemon.plist 2>/dev/null
launchctl unload /Library/LaunchDaemons/com.proidentity.app.plist 2>/dev/null
rm -f /Library/LaunchDaemons/com.proitservices.proidentity.access.daemon.plist
rm -f /Library/LaunchDaemons/com.proidentity.app.plist
# Stop and remove UI launch agent for all users
for uid in $(ls /var/folders/ 2>/dev/null | head -1 || true); do true; done
CONSOLE_USER=$(stat -f "%Su" /dev/console 2>/dev/null || true)
if [ -n "$CONSOLE_USER" ] && [ "$CONSOLE_USER" != "root" ]; then
    CONSOLE_UID=$(id -u "$CONSOLE_USER" 2>/dev/null || true)
    if [ -n "$CONSOLE_UID" ]; then
        launchctl bootout gui/"$CONSOLE_UID" /Library/LaunchAgents/com.proitservices.proidentity.access-ui.plist 2>/dev/null || true
        launchctl bootout gui/"$CONSOLE_UID" /Library/LaunchAgents/com.proidentity.app-ui.plist 2>/dev/null || true
    fi
fi
rm -f /Library/LaunchAgents/com.proitservices.proidentity.access-ui.plist
rm -f /Library/LaunchAgents/com.proidentity.app-ui.plist
rm -rf /Library/ProIdentity
rm -rf "/Applications/ProIdentity Access.app"
rm -rf /Applications/ProIdentity.app
rm -rf "/Applications/ProIdentity Access Uninstaller.app"
rm -rf "/Applications/ProIdentity Uninstaller.app"
rm -rf /Library/Logs/ProIdentity
`
	if !keepData {
		script += `rm -rf "/Library/Application Support/ProIdentity"` + "\n"
	}

	tmp, err := os.CreateTemp("", "proidentity-uninstall-*.sh")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}

	out, err := exec.Command("osascript", "-e",
		fmt.Sprintf(`do shell script "%s" with administrator privileges`, tmpPath),
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("uninstall failed: %w\n%s", err, out)
	}

	runtime.Quit(a.ctx)
	return nil
}
