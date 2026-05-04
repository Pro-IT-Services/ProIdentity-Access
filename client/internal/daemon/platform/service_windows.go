//go:build windows

package platform

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const ServiceName = "ProIdentity"

// DaemonMain is the entry point called from cmd/daemon/main.go.
// It runs as a Windows Service when started by the SCM, or in the
// foreground (console mode) when run directly.
func DaemonMain(run func() error, stop func()) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("check service mode: %w", err)
	}

	if isService {
		return svc.Run(ServiceName, &handler{run: run, stop: stop})
	}

	// Console mode — run directly
	log.Println("Running in console mode (not as Windows Service)")
	return run()
}

// handler implements svc.Handler.
type handler struct {
	run  func() error
	stop func()
}

func (h *handler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	s <- svc.Status{State: svc.StartPending}

	errCh := make(chan error, 1)
	go func() {
		errCh <- h.run()
	}()

	s <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				log.Printf("daemon error: %v", err)
				return true, 1
			}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				h.stop()
				select {
				case <-errCh:
				case <-time.After(10 * time.Second):
				}
				return false, 0
			}
		}
	}
}

// Install registers the daemon as a Windows Service.
func Install(exePath string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", ServiceName)
	}

	s, err = m.CreateService(ServiceName, exePath, mgr.Config{
		DisplayName: "ProIdentity Access Daemon",
		Description: "Runs privileged VPN tunnel operations for ProIdentity Access.",
		StartType:   mgr.StartAutomatic,
	}, "daemon")
	if err != nil {
		return err
	}
	defer s.Close()

	err = eventlog.InstallAsEventCreate(ServiceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		log.Printf("warn: install event log: %v", err)
	}
	return nil
}

// Uninstall removes the Windows Service registration.
func Uninstall() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(ServiceName)
	if err != nil {
		return fmt.Errorf("service %s not found: %w", ServiceName, err)
	}
	defer s.Close()

	return s.Delete()
}
