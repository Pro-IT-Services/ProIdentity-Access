//go:build darwin

package platform

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

const ServiceName = "com.proitservices.proidentity.access.daemon"

// DaemonMain is the entry point called from cmd/daemon/main.go.
// On macOS it blocks until SIGTERM or SIGINT, then calls stop().
func DaemonMain(run func() error, stop func()) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- run()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("Received shutdown signal")
		stop()
		return <-errCh
	}
}

// Install writes the LaunchDaemon plist to /Library/LaunchDaemons/.
func Install(exePath string) error {
	const plistPath = "/Library/LaunchDaemons/com.proitservices.proidentity.access.daemon.plist"
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.proitservices.proidentity.access.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>` + exePath + `</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>/Library/Logs/ProIdentity/daemon.log</string>
    <key>StandardOutPath</key>
    <string>/Library/Logs/ProIdentity/daemon.log</string>
</dict>
</plist>`

	return os.WriteFile(plistPath, []byte(plist), 0644)
}

// Uninstall removes the LaunchDaemon plist.
func Uninstall() error {
	var lastErr error
	for _, path := range []string{
		"/Library/LaunchDaemons/com.proitservices.proidentity.access.daemon.plist",
		"/Library/LaunchDaemons/com.proidentity.app.plist",
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}
	return lastErr
}
