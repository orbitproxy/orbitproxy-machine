//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	darwinServiceLabel = "com.orbitproxy.machine"
	darwinPlistPath    = "/Library/LaunchDaemons/com.orbitproxy.machine.plist"
)

func requireAdmin() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("installing or managing the service requires root. Run with sudo")
	}
	return nil
}

func installPlatformService(args serviceInstallArgs) error {
	if _, err := os.Stat(darwinPlistPath); err == nil {
		return alreadyInstalledError(darwinPlistPath)
	}

	if err := os.WriteFile(darwinPlistPath, []byte(darwinPlist(args)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", darwinPlistPath, err)
	}
	if err := runCommand("launchctl", "load", "-w", darwinPlistPath); err != nil {
		return err
	}
	fmt.Printf("OrbitProxy service installed and started.\nPlist: %s\nUser: %s\nWorkdir: %s\n", darwinPlistPath, args.Username, args.Workdir)
	return nil
}

func darwinPlist(args serviceInstallArgs) string {
	userName := args.Username
	if userName == "" || userName == "root" {
		userName = ""
	}
	userXML := ""
	if userName != "" {
		userXML = fmt.Sprintf("\t<key>UserName</key>\n\t<string>%s</string>\n", xmlEscape(userName))
	}
	envXML := darwinEnvXML(args.Home, args.Path)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--workdir</string>
		<string>%s</string>
		<string>run</string>
		<string>--machine-key</string>
		<string>%s</string>
		<string>--from-service</string>
	</array>
%s%s	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>ThrottleInterval</key>
	<integer>5</integer>
</dict>
</plist>
`, darwinServiceLabel, xmlEscape(args.Exe), xmlEscape(args.Workdir), xmlEscape(args.MachineKey), userXML, envXML)
}

func darwinEnvXML(home, pathEnv string) string {
	if home == "" && pathEnv == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
	if home != "" {
		fmt.Fprintf(&b, "\t\t<key>HOME</key>\n\t\t<string>%s</string>\n", xmlEscape(home))
	}
	if pathEnv != "" {
		fmt.Fprintf(&b, "\t\t<key>PATH</key>\n\t\t<string>%s</string>\n", xmlEscape(pathEnv))
	}
	b.WriteString("\t</dict>\n")
	return b.String()
}

func xmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	)
	return replacer.Replace(s)
}

func platformServiceInstalled() (bool, error) {
	_, err := os.Stat(darwinPlistPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat %s: %w", darwinPlistPath, err)
}

func removePlatformService() error {
	_ = exec.Command("launchctl", "unload", "-w", darwinPlistPath).Run()
	if _, err := os.Stat(darwinPlistPath); os.IsNotExist(err) {
		return nil
	}
	if err := os.Remove(darwinPlistPath); err != nil {
		return fmt.Errorf("remove %s: %w", darwinPlistPath, err)
	}
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
	if err := runCommand("launchctl", "load", "-w", darwinPlistPath); err != nil {
		return err
	}
	fmt.Println("OrbitProxy service started.")
	return nil
}

func stopPlatformService() error {
	if err := runCommand("launchctl", "unload", "-w", darwinPlistPath); err != nil {
		return err
	}
	fmt.Println("OrbitProxy service stopped.")
	return nil
}

func restartPlatformService() error {
	_ = exec.Command("launchctl", "unload", "-w", darwinPlistPath).Run()
	if err := runCommand("launchctl", "load", "-w", darwinPlistPath); err != nil {
		return err
	}
	fmt.Println("OrbitProxy service restarted.")
	return nil
}

func statusPlatformService() (string, error) {
	cmd := exec.Command("launchctl", "list", darwinServiceLabel)
	output, err := cmd.CombinedOutput()
	if err != nil {
		out := string(output)
		if strings.Contains(out, "Could not find service") || strings.Contains(out, "Could not find specified service") {
			return "Service status: not running", nil
		}
		if _, statErr := os.Stat(darwinPlistPath); os.IsNotExist(statErr) {
			return "Service status: not installed", nil
		}
		return "Service status: not running", nil
	}
	_ = output
	return "Service status: running", nil
}
