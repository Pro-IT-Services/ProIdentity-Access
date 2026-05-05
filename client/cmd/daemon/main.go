package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"wg-client/internal/daemon"
	"wg-client/internal/daemon/platform"
	"wg-client/internal/ipc"
)

func main() {
	install := flag.Bool("install", false, "Install the daemon as a system service")
	uninstall := flag.Bool("uninstall", false, "Uninstall the system service")
	flag.Parse()

	if *install {
		exe, _ := os.Executable()
		if err := platform.Install(exe); err != nil {
			log.Fatalf("install: %v", err)
		}
		log.Println("Service installed successfully")
		return
	}

	if *uninstall {
		if err := platform.Uninstall(); err != nil {
			log.Fatalf("uninstall: %v", err)
		}
		log.Println("Service uninstalled successfully")
		return
	}

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Printf("ProIdentity Access Daemon starting (platform: %s)", runtime.GOOS)

	// Extract wintun.dll next to the executable before any WireGuard init (Windows only).
	if err := daemon.EnsureWintun(); err != nil {
		log.Fatalf("extract wintun.dll: %v", err)
	}

	storageDir := configStorageDir()
	log.Printf("Config storage: %s", storageDir)

	var server *ipc.Server

	manager, err := daemon.NewTunnelManager(storageDir, func(evt ipc.Event) {
		if server != nil {
			server.Broadcast(evt)
		}
	})
	if err != nil {
		log.Fatalf("init tunnel manager: %v", err)
	}

	// IPC authorization is based on OS peer identity and per-user ownership.
	// Stale legacy token files are removed so old GUI clients do not depend
	// on a machine-wide shared secret.
	ipc.RemoveTokenFile()
	server = ipc.NewServer(manager, "")

	runFn := func() error {
		if err := server.Start(); err != nil {
			return err
		}
		log.Println("Daemon ready")
		// Block until stopped
		select {}
	}

	stopFn := func() {
		log.Println("Shutting down...")
		manager.StopAll()
		server.Stop()
		ipc.RemoveTokenFile()
	}

	if err := platform.DaemonMain(runFn, stopFn); err != nil {
		log.Fatalf("daemon: %v", err)
	}
}

func configStorageDir() string {
	switch runtime.GOOS {
	case "windows":
		dir := os.Getenv("ProgramData")
		if dir == "" {
			dir = `C:\ProgramData`
		}
		return filepath.Join(dir, "ProIdentity", "tunnels")
	case "darwin":
		return "/Library/Application Support/ProIdentity/tunnels"
	default:
		return "/etc/proidentity/tunnels"
	}
}
