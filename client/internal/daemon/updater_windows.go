//go:build windows

package daemon

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"wg-client/internal/ipc"
)

const updatesSupported = true

// prepareUpdateDir creates dir so that only SYSTEM and Administrators can
// write to it: a user can't swap the verified package before msiexec runs.
func prepareUpdateDir(dir string) error {
	if fi, err := os.Lstat(dir); err == nil && (fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || !fi.IsDir()) {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return applyFileSDDL(dir, "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)")
}

const (
	createBreakawayFromJob = 0x01000000
	detachedProcess        = 0x00000008
	msiSuccessReboot       = 3010
	msiSuccessRebootInit   = 1641
)

// runInstaller starts a silent per-machine MSI upgrade. msiexec runs detached
// so it survives the MSI stopping this service; the new service starts at the
// end of the upgrade and relaunches the app (ResumeAfterUpdate).
func runInstaller(pkg, version, dir string, principals []ipc.Principal, onFail func(error)) error {
	writeRelaunchMarker(dir, version, principals)

	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	msiexec := filepath.Join(sysRoot, "System32", "msiexec.exe")
	logPath := filepath.Join(dir, "install-"+version+".log")
	args := []string{"/i", pkg, "/qn", "/norestart", "/l*v", logPath}

	start := func(flags uint32) (*exec.Cmd, error) {
		cmd := exec.Command(msiexec, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: flags}
		return cmd, cmd.Start()
	}
	cmd, err := start(detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP | createBreakawayFromJob)
	if err != nil {
		// Not in a job that allows breakaway; start without it.
		cmd, err = start(detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP)
	}
	if err != nil {
		os.Remove(filepath.Join(dir, relaunchMarker))
		return fmt.Errorf("start installer: %w", err)
	}
	log.Printf("update: msiexec started (pid %d), log %s", cmd.Process.Pid, logPath)

	go func() {
		err := cmd.Wait()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			switch exitErr.ExitCode() {
			case msiSuccessReboot, msiSuccessRebootInit:
				return
			}
		}
		if err != nil {
			onFail(err)
		}
	}()
	return nil
}

// relaunchApp starts the app again in each active session of the given users
// (they had it open when the update started). The app is single-instance, so
// launching it where it already runs only brings it forward.
func relaunchApp(userIDs []string) {
	if len(userIDs) == 0 {
		return
	}
	want := map[string]bool{}
	for _, id := range userIDs {
		want[id] = true
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	app := filepath.Join(filepath.Dir(self), "ProIdentity Access.exe")
	if _, err := os.Stat(app); err != nil {
		log.Printf("update: app not found for relaunch: %v", err)
		return
	}

	launched := map[uint32]bool{}
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		for _, session := range activeSessions() {
			if launched[session] {
				continue
			}
			var token windows.Token
			if err := windows.WTSQueryUserToken(session, &token); err != nil {
				continue
			}
			sid := tokenSID(token)
			if want[sid] {
				if err := launchAsUser(token, app); err != nil {
					log.Printf("update: relaunch in session %d: %v", session, err)
				} else {
					log.Printf("update: relaunched app in session %d", session)
				}
				launched[session] = true
				delete(want, sid)
			}
			token.Close()
		}
		if len(want) == 0 {
			return
		}
		time.Sleep(5 * time.Second)
	}
}

func activeSessions() []uint32 {
	var info *windows.WTS_SESSION_INFO
	var count uint32
	if err := windows.WTSEnumerateSessions(0, 0, 1, &info, &count); err != nil {
		return nil
	}
	defer windows.WTSFreeMemory(uintptr(unsafe.Pointer(info)))
	list := unsafe.Slice(info, count)
	var out []uint32
	for _, s := range list {
		if s.State == windows.WTSActive {
			out = append(out, s.SessionID)
		}
	}
	return out
}

func tokenSID(t windows.Token) string {
	u, err := t.GetTokenUser()
	if err != nil {
		return ""
	}
	return u.User.Sid.String()
}

func launchAsUser(token windows.Token, app string) error {
	var env *uint16
	if err := windows.CreateEnvironmentBlock(&env, token, false); err != nil {
		return err
	}
	defer windows.DestroyEnvironmentBlock(env)

	cmdLine, err := windows.UTF16PtrFromString(`"` + app + `"`)
	if err != nil {
		return err
	}
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(app))
	desktop, _ := windows.UTF16PtrFromString(`winsta0\default`)
	si := windows.StartupInfo{Desktop: desktop}
	si.Cb = uint32(unsafe.Sizeof(si))
	var pi windows.ProcessInformation
	err = windows.CreateProcessAsUser(token, nil, cmdLine, nil, nil, false,
		windows.CREATE_UNICODE_ENVIRONMENT|windows.CREATE_NEW_PROCESS_GROUP, env, dir, &si, &pi)
	if err != nil {
		return err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)
	return nil
}
