//go:build windows

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func hardenStorageACL(storageDir string) error {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = `C:\ProgramData`
	}
	absStorage, err := filepath.Abs(storageDir)
	if err != nil {
		return err
	}
	absProgramData, err := filepath.Abs(programData)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(strings.ToLower(absStorage), strings.ToLower(absProgramData)+string(filepath.Separator)) {
		return nil
	}

	parent := filepath.Dir(storageDir)
	if err := applyFileSDDL(parent, "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GRGX;;;BU)"); err != nil {
		return fmt.Errorf("program data dir: %w", err)
	}
	if err := applyFileSDDL(storageDir, "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"); err != nil {
		return fmt.Errorf("tunnel storage dir: %w", err)
	}
	return nil
}

func applyFileSDDL(path, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	)
}
