//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	windowsServiceName = "OrbitProxy"
	windowsServiceDesc = "OrbitProxy machine client"
)

func requireAdmin() error {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("check administrator privileges: %w", err)
	}
	defer token.Close()
	if !token.IsElevated() {
		return fmt.Errorf("managing the service requires administrator privileges")
	}
	return nil
}

func maybeRunAsService() bool {
	isInt, err := svc.IsAnInteractiveSession()
	if err != nil || isInt {
		return false
	}
	err = svc.Run(windowsServiceName, &windowsService{})
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == 1063 {
			return false
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return true
}

type windowsService struct{}

func (m *windowsService) Execute(serviceArgs []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if len(serviceArgs) > 1 {
		rootCmd.SetArgs(serviceArgs[1:])
	}

	errC := make(chan error, 1)
	go func() {
		errC <- rootCmd.ExecuteContext(ctx)
	}()

	status <- svc.Status{State: svc.StartPending}
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				select {
				case <-errC:
				case <-time.After(15 * time.Second):
				}
				return false, 0
			}
		case err := <-errC:
			if err != nil {
				return true, 1
			}
			return false, 0
		}
	}
}

func installPlatformService(args serviceInstallArgs) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err == nil {
		s.Close()
		return alreadyInstalledError(windowsServiceName)
	}

	s, err = m.CreateService(windowsServiceName, args.Exe, mgr.Config{
		DisplayName: windowsServiceDesc,
		Description: "OrbitProxy machine client daemon",
		StartType:   mgr.StartAutomatic,
	}, "--workdir", args.Workdir, "run", "--machine-key", args.MachineKey, "--from-service")
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()

	_ = eventlog.InstallAsEventCreate(windowsServiceName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	fmt.Printf("OrbitProxy service installed and started.\nWorkdir: %s\n", args.Workdir)
	return nil
}

func platformServiceInstalled() (bool, error) {
	m, err := mgr.Connect()
	if err != nil {
		return false, fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return false, nil
	}
	s.Close()
	return true, nil
}

func removePlatformService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return nil
	}
	defer s.Close()

	_, _ = s.Control(svc.Stop)
	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	_ = eventlog.Remove(windowsServiceName)
	return nil
}

func uninstallPlatformService() error {
	installed, err := platformServiceInstalled()
	if err != nil {
		return err
	}
	if !installed {
		return fmt.Errorf("OrbitProxy service is not installed")
	}
	if err := removePlatformService(); err != nil {
		return err
	}
	fmt.Println("OrbitProxy service uninstalled. Local config under ~/.orbitproxy was not removed.")
	return nil
}

func startPlatformService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return fmt.Errorf("OrbitProxy service is not installed")
	}
	defer s.Close()
	if err := s.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	fmt.Println("OrbitProxy service started.")
	return nil
}

func stopPlatformService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return fmt.Errorf("OrbitProxy service is not installed")
	}
	defer s.Close()
	if _, err := s.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}
	fmt.Println("OrbitProxy service stopped.")
	return nil
}

func restartPlatformService() error {
	if err := stopPlatformService(); err != nil {
		return err
	}
	return startPlatformService()
}

func statusPlatformService() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("connect to service manager: %w", err)
	}
	defer m.Disconnect()
	s, err := m.OpenService(windowsServiceName)
	if err != nil {
		return "Service status: not installed", nil
	}
	defer s.Close()
	st, err := s.Query()
	if err != nil {
		return "", fmt.Errorf("query service: %w", err)
	}
	switch st.State {
	case svc.Running:
		return "Service status: running", nil
	case svc.Stopped:
		return "Service status: stopped", nil
	default:
		return fmt.Sprintf("Service status: %v", st.State), nil
	}
}
