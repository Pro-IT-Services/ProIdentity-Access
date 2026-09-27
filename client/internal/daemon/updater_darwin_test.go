//go:build darwin

package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerJobPlistIsValid(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "install & <log>.log")
	data := launchdPlist(updaterLabel, []string{"/usr/sbin/installer", "-pkg", "/tmp/a&b.pkg", "-target", "/"}, logPath)
	path := filepath.Join(t.TempDir(), "job.plist")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("/usr/bin/plutil", "-lint", path).CombinedOutput()
	if err != nil {
		t.Fatalf("plutil rejected the job plist: %s", out)
	}
	args, err := exec.Command("/usr/bin/plutil", "-extract", "ProgramArguments.2", "raw", path).Output()
	if err != nil || strings.TrimSpace(string(args)) != "/tmp/a&b.pkg" {
		t.Fatalf("package argument not preserved: %q (%v)", args, err)
	}
}
