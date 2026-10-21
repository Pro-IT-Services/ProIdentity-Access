package main

// PlaceTrayWindow sizes the tray popover to width×height (logical pixels) and
// puts it in the bottom-right corner of the primary monitor's work area, so it
// never straddles two screens or hides behind the taskbar. It reports false
// when the platform has no native placement; the frontend then falls back to
// the Wails runtime positioning.
func (a *App) PlaceTrayWindow(width, height int) bool {
	return placeTrayWindow(width, height)
}
