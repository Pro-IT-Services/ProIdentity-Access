//go:build !darwin && !windows

package main

import "context"

func (a *App) setupTray(ctx context.Context) {}
func (a *App) teardownTray()                 {}
func signalTrayRefresh()                     {}
