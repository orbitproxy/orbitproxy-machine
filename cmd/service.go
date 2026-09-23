package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/runlock"
	"github.com/spf13/cobra"
)

var (
	serviceMachineKeyFlag string
	errServiceUnsupported = fmt.Errorf("orbitproxy service is not supported on this operating system")
)

type serviceInstallArgs struct {
	Exe        string
	Workdir    string
	MachineKey string
	Username   string
	Group      string
	Home       string
	Path       string
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Install and manage the OrbitProxy OS service",
	Long:  "Register OrbitProxy as a native OS service so it starts on boot. Configuration stays in ~/.orbitproxy; sudo is only required to register the service.",
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and start the OrbitProxy service",
	RunE:  serviceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Stop and remove the OrbitProxy service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAdmin(); err != nil {
			return err
		}
		return uninstallPlatformService()
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the OrbitProxy service",
	RunE:  serviceStart,
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the OrbitProxy service",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireAdmin(); err != nil {
			return err
		}
		return stopPlatformService()
	},
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart the OrbitProxy service",
	RunE:  serviceRestart,
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show OrbitProxy service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := statusPlatformService()
		if err != nil {
			return err
		}
		fmt.Println(status)
		return nil
	},
}

func init() {
	serviceCmd.PersistentFlags().StringVar(&serviceMachineKeyFlag, "machine-key", "", "machine key under --workdir (required when multiple machines exist)")
	serviceCmd.AddCommand(serviceInstallCmd, serviceUninstallCmd, serviceStartCmd, serviceStopCmd, serviceRestartCmd, serviceStatusCmd)
	rootCmd.AddCommand(serviceCmd)
}

func serviceInstall(cmd *cobra.Command, _ []string) error {
	if err := requireAdmin(); err != nil {
		return err
	}

	args, machineDir, err := resolveServiceInstallArgs()
	if err != nil {
		return err
	}
	if err := ensureNotRunning(machineDir); err != nil {
		return err
	}
	offset := logFileSize(config.LogPath(machineDir))
	if err := installPlatformService(args); err != nil {
		return err
	}
	return reportServiceConnect(cmd.OutOrStdout(), machineDir, offset)
}

func serviceStart(cmd *cobra.Command, _ []string) error {
	if err := requireAdmin(); err != nil {
		return err
	}
	_, machineDir, err := resolveServiceInstallArgs()
	if err != nil {
		return err
	}
	if err := ensureForegroundStopped(machineDir); err != nil {
		return err
	}
	offset := logFileSize(config.LogPath(machineDir))
	if err := startPlatformService(); err != nil {
		return err
	}
	return reportServiceConnect(cmd.OutOrStdout(), machineDir, offset)
}

func serviceRestart(cmd *cobra.Command, _ []string) error {
	if err := requireAdmin(); err != nil {
		return err
	}
	_, machineDir, err := resolveServiceInstallArgs()
	if err != nil {
		return err
	}
	offset := logFileSize(config.LogPath(machineDir))
	if err := restartPlatformService(); err != nil {
		return err
	}
	return reportServiceConnect(cmd.OutOrStdout(), machineDir, offset)
}

func resolveServiceInstallArgs() (serviceInstallArgs, string, error) {
	owner, err := config.ResolveServiceOwner()
	if err != nil {
		return serviceInstallArgs{}, "", err
	}
	root, err := config.ResolveServiceRoot(workDirFlag, owner)
	if err != nil {
		return serviceInstallArgs{}, "", err
	}
	machineDir, err := config.ResolveMachineDir(root, serviceMachineKeyFlag)
	if err != nil {
		return serviceInstallArgs{}, "", err
	}
	machineCfg, err := config.Load(machineDir)
	if err != nil {
		return serviceInstallArgs{}, "", err
	}
	exe, err := executablePath()
	if err != nil {
		return serviceInstallArgs{}, "", err
	}
	args := serviceInstallArgs{
		Exe:        exe,
		Workdir:    root,
		MachineKey: machineCfg.MachineKey,
		Username:   owner.Username,
		Group:      owner.Group,
		Home:       owner.Home,
		Path:       os.Getenv("PATH"),
	}
	if err := writeServiceMcpRuntime(machineDir, args); err != nil {
		return serviceInstallArgs{}, "", err
	}
	return args, machineDir, nil
}

func writeServiceMcpRuntime(machineDir string, args serviceInstallArgs) error {
	if err := config.WriteMcpRuntime(machineDir, config.McpRuntime{
		Home:     args.Home,
		Path:     args.Path,
		NpmRootG: probeNpmRootG(args.Home, args.Path),
	}); err != nil {
		return err
	}
	return config.OwnServiceTree(machineDir, &config.ServiceOwner{
		Username: args.Username,
		Group:    args.Group,
		Home:     args.Home,
	})
}

func probeNpmRootG(home, pathEnv string) string {
	cmd := exec.Command("npm", "root", "-g")
	env := os.Environ()
	if pathEnv != "" {
		env = replaceInstallEnv(env, "PATH", pathEnv)
	}
	if home != "" {
		env = replaceInstallEnv(env, "HOME", home)
	}
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func replaceInstallEnv(env []string, key, val string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			if !replaced {
				out = append(out, prefix+val)
				replaced = true
			}
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, prefix+val)
	}
	return out
}

func ensureNotRunning(machineDir string) error {
	holder, err := runlock.Inspect(machineDir)
	if err != nil {
		return err
	}
	if holder == nil {
		return nil
	}
	return fmt.Errorf("%s", runlock.FormatConflict(*holder))
}

func ensureForegroundStopped(machineDir string) error {
	holder, err := runlock.Inspect(machineDir)
	if err != nil {
		return err
	}
	if holder == nil || holder.Mode == runlock.ModeService {
		return nil
	}
	return fmt.Errorf("%s", runlock.FormatConflict(*holder))
}

func executablePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return filepath.Abs(p)
}

func alreadyInstalledError(path string) error {
	return fmt.Errorf("OrbitProxy service is already installed at %s\nReinstall: orbitproxy service uninstall\nStart:     orbitproxy service start", path)
}
