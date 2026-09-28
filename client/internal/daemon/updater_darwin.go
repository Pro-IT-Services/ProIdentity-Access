//go:build darwin

package daemon

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"wg-client/internal/ipc"
)

const updatesSupported = true

const (
	updaterLabel = "com.proitservices.proidentity.access.updater"
	uiAgentLabel = "com.proitservices.proidentity.access.ui"
)

// prepareUpdateDir creates dir owned by root with mode 0700, so no user can
// swap the verified package before installer(8) runs.
func prepareUpdateDir(dir string) error {
	if fi, err := os.Lstat(dir); err == nil && (fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir()) {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chown(dir, 0, 0); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// runInstaller runs installer(8) as its own launchd job: the package's
// preinstall script stops this daemon, which would take a child process down
// with it. postinstall restarts the daemon and the app.
func runInstaller(pkg, version, dir string, principals []ipc.Principal, onFail func(error)) error {
	writeRelaunchMarker(dir, version, principals)

	logPath := filepath.Join(dir, "install-"+version+".log")
	plistPath := filepath.Join(dir, updaterLabel+".plist")
	plist := launchdPlist(updaterLabel, []string{"/usr/sbin/installer", "-pkg", pkg, "-target", "/"}, logPath)
	if err := os.WriteFile(plistPath, plist, 0o644); err != nil {
		return err
	}
	if err := os.Chown(plistPath, 0, 0); err != nil {
		return err
	}

	// A job left from an earlier update would block bootstrap.
	_ = exec.Command("/bin/launchctl", "bootout", "system/"+updaterLabel).Run()
	if out, err := exec.Command("/bin/launchctl", "bootstrap", "system", plistPath).CombinedOutput(); err != nil {
		os.Remove(filepath.Join(dir, relaunchMarker))
		return fmt.Errorf("start installer: %v: %s", err, bytes.TrimSpace(out))
	}
	log.Printf("update: installer job started, log %s", logPath)
	return nil
}

func launchdPlist(label string, args []string, logPath string) []byte {
	esc := func(s string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>` + esc(label) + `</string>
  <key>ProgramArguments</key>
  <array>
`)
	for _, a := range args {
		b.WriteString("    <string>" + esc(a) + "</string>\n")
	}
	b.WriteString(`  </array>
  <key>RunAtLoad</key><true/>
  <key>AbandonProcessGroup</key><true/>
  <key>StandardOutPath</key><string>` + esc(logPath) + `</string>
  <key>StandardErrorPath</key><string>` + esc(logPath) + `</string>
</dict>
</plist>
`)
	return b.Bytes()
}

// activeUserIDs returns the uid of the user at the console (the one who can
// see a prompt), if any.
func activeUserIDs() []string {
	var st syscall.Stat_t
	if err := syscall.Stat("/dev/console", &st); err != nil || st.Uid == 0 {
		return nil
	}
	return []string{strconv.FormatUint(uint64(st.Uid), 10)}
}

// relaunchApp starts the app's launch agent for users who had it open. After
// a successful update postinstall already did this; kickstart without -k
// leaves a running app alone.
func relaunchApp(userIDs []string) {
	for _, uid := range userIDs {
		if err := exec.Command("/bin/launchctl", "kickstart", "gui/"+uid+"/"+uiAgentLabel).Run(); err != nil {
			log.Printf("update: relaunch for uid %s: %v", uid, err)
		}
	}
}
