//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	linuxServiceName = "orbitproxy"
	linuxServicePath = "/etc/systemd/system/orbitproxy.service"
)

func requireAdmin() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("installing or managing the service requires root. Run with sudo")
	}
	return nil
}

func installPlatformService(args serviceInstallArgs) error {
	if _, err := os.Stat(linuxServicePath); err == nil {
		return alreadyInstalledError(linuxServicePath)
	}

	unit := linuxUnit(args)
	if err := os.WriteFile(linuxServicePath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", linuxServicePath, err)
	}
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := runCommand("systemctl", "enable", "--now", linuxServiceName); err != nil {
		return withLinuxJournal(err)
	}
	fmt.Printf("OrbitProxy service installed and started.\nUnit: %s\nUser: %s\nWorkdir: %s\n", linuxServicePath, args.Username, args.Workdir)
	return nil
}

func linuxUnit(args serviceInstallArgs) string {
	execStart := strings.Join([]string{
		strconv.Quote(args.Exe),
		"--workdir", strconv.Quote(args.Workdir),
		"run",
		"--machine-key", strconv.Quote(args.MachineKey),
		"--from-service",
	}, " ")

	var extra strings.Builder
	if args.Username != "" && args.Username != "root" {
		extra.WriteString("User=" + args.Username + "\n")
		if args.Group != "" {
			extra.WriteString("Group=" + args.Group + "\n")
		}
	}
	extra.WriteString(systemdEnvLine("HOME", args.Home))
	extra.WriteString(systemdEnvLine("PATH", args.Path))

	return fmt.Sprintf(`[Unit]
Description=OrbitProxy machine client
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
%sExecStart=%s
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`, extra.String(), execStart)
}

func systemdEnvLine(key, val string) string {
	if val == "" {
		return ""
	}
	return "Environment=" + strconv.Quote(key+"="+val) + "\n"
}

func platformServiceInstalled() (bool, error) {
	_, err := os.Stat(linuxServicePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", linuxServicePath, err)
}

func removePlatformService() error {
	_ = exec.Command("systemctl", "stop", linuxServiceName).Run()
	_ = exec.Command("systemctl", "disable", linuxServiceName).Run()
	if _, err := os.Stat(linuxServicePath); os.IsNotExist(err) {
		return nil
	}
	if err := os.Remove(linuxServicePath); err != nil {
		return fmt.Errorf("remove %s: %w", linuxServicePath, err)
	}
	return runCommand("systemctl", "daemon-reload")
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
	if err := runCommand("systemctl", "enable", "--now", linuxServiceName); err != nil {
		return withLinuxJournal(err)
	}
	fmt.Println("OrbitProxy service started.")
	return nil
}

func stopPlatformService() error {
	if err := runCommand("systemctl", "stop", linuxServiceName); err != nil {
		return err
	}
	fmt.Println("OrbitProxy service stopped.")
	return nil
}

func restartPlatformService() error {
	if err := runCommand("systemctl", "restart", linuxServiceName); err != nil {
		return withLinuxJournal(err)
	}
	fmt.Println("OrbitProxy service restarted.")
	return nil
}

func withLinuxJournal(err error) error {
	out, jerr := exec.Command("journalctl", "-u", linuxServiceName, "-n", "40", "--no-pager").CombinedOutput()
	if jerr != nil || len(out) == 0 {
		return err
	}
	return fmt.Errorf("%w\n%s", err, string(out))
}

func statusPlatformService() (string, error) {
	cmd := exec.Command("systemctl", "is-active", linuxServiceName)
	output, err := cmd.CombinedOutput()
	status := strings.TrimSpace(string(output))
	if status == "" && err != nil {
		return "", fmt.Errorf("query service status: %w", err)
	}
	if status == "" {
		status = "unknown"
	}
	return "Service status: " + status, nil
}
