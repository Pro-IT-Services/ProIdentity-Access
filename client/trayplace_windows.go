//go:build windows

package main

import (
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	shcore                = windows.NewLazySystemDLL("shcore.dll")
	procEnumWindows       = user32.NewProc("EnumWindows")
	procMonitorFromPoint  = user32.NewProc("MonitorFromPoint")
	procGetMonitorInfoW   = user32.NewProc("GetMonitorInfoW")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procGetDpiForMonitor  = shcore.NewProc("GetDpiForMonitor")
	findWindowMu          sync.Mutex
	foundHwnd             windows.HWND
	enumOurWindowCallback = windows.NewCallback(enumOurWindow)
)

const (
	monitorDefaultToPrimary = 1
	mdtEffectiveDPI         = 0
	swpNoZOrder             = 0x0004
	swpNoActivate           = 0x0010
	trayMargin              = 12 // logical px between the window and the work-area edge
)

type rect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

// enumOurWindow stops at the first top-level Wails window owned by this process.
// Matching by class instead of title: the title changes (RoboForm autofill).
func enumOurWindow(hwnd windows.HWND, _ uintptr) uintptr {
	var pid uint32
	windows.GetWindowThreadProcessId(hwnd, &pid)
	if pid != uint32(os.Getpid()) {
		return 1
	}
	buf := make([]uint16, 64)
	n, err := windows.GetClassName(hwnd, &buf[0], int32(len(buf)))
	if err != nil || windows.UTF16ToString(buf[:n]) != "wailsWindow" {
		return 1
	}
	foundHwnd = hwnd
	return 0
}

func ourWindow() windows.HWND {
	findWindowMu.Lock()
	defer findWindowMu.Unlock()
	foundHwnd = 0
	procEnumWindows.Call(enumOurWindowCallback, 0)
	return foundHwnd
}

func placeTrayWindow(width, height int) bool {
	defer func() { recover() }() // never let window placement crash the app
	hwnd := ourWindow()
	if hwnd == 0 {
		return false
	}

	// Primary monitor and its work area (the screen minus the taskbar, wherever it is).
	hmon, _, _ := procMonitorFromPoint.Call(0, monitorDefaultToPrimary) // POINT{0,0}
	if hmon == 0 {
		return false
	}
	mi := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := procGetMonitorInfoW.Call(hmon, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		return false
	}

	// Scale logical px to the primary monitor's physical px.
	scale := 1.0
	if procGetDpiForMonitor.Find() == nil {
		var dpiX, dpiY uint32
		if hr, _, _ := procGetDpiForMonitor.Call(hmon, mdtEffectiveDPI,
			uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY))); hr == 0 && dpiX > 0 {
			scale = float64(dpiX) / 96.0
		}
	}
	w := int32(float64(width) * scale)
	h := int32(float64(height) * scale)
	margin := int32(float64(trayMargin) * scale)

	work := mi.RcWork
	x := work.Right - w - margin
	y := work.Bottom - h - margin
	if x < work.Left+margin {
		x = work.Left + margin
	}
	if y < work.Top+margin {
		y = work.Top + margin
	}

	// Twice: moving onto a monitor with a different DPI makes Windows rescale
	// the window, so the second call applies the final size and position.
	for i := 0; i < 2; i++ {
		procSetWindowPos.Call(uintptr(hwnd), 0,
			uintptr(int(x)), uintptr(int(y)), uintptr(int(w)), uintptr(int(h)),
			swpNoZOrder|swpNoActivate)
	}
	return true
}
