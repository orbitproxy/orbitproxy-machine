package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/runlock"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show local machine config without starting the client",
	Long:  "Read ~/.orbitproxy (or --workdir) and print each machine that has orbitproxy.yml. Does not connect or take the run lock.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, _ []string) error {
	return printLocalStatus(cmd.OutOrStdout(), workDirFlag)
}

func printLocalStatus(out io.Writer, workdir string) error {
	root, err := config.ResolveRoot(workdir)
	if err != nil {
		return err
	}
	keys, err := config.ListMachineKeys(root)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("no machine config under %s; run `orbitproxy authtoken` first", root)
	}

	fmt.Fprintf(out, "root      %s\n", root)
	fmt.Fprintf(out, "version   %s\n", clientVersion())
	fmt.Fprintf(out, "machines  %d\n", len(keys))
	for _, key := range keys {
		fmt.Fprintln(out)
		if err := writeMachineStatus(out, root, key); err != nil {
			return err
		}
	}
	return nil
}

func writeMachineStatus(out io.Writer, root, key string) error {
	machineDir, err := config.MachineDir(root, key)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "machine   %s\n", key)
	fmt.Fprintf(out, "dir       %s\n", machineDir)
	fmt.Fprintf(out, "config    %s\n", config.Path(machineDir))
	fmt.Fprintf(out, "logs      %s\n", config.LogsDir(machineDir))

	cfg, err := config.Load(machineDir)
	if err != nil {
		fmt.Fprintf(out, "error     %s\n", err)
		fmt.Fprintf(out, "process   %s\n", inspectProcess(machineDir))
		return nil
	}
	kind := strings.TrimSpace(cfg.Type)
	if kind == "" {
		kind = "-"
	}
	edge := strings.TrimSpace(cfg.EdgeAddr)
	if edge == "" {
		edge = "-"
	}
	fmt.Fprintf(out, "type      %s\n", kind)
	fmt.Fprintf(out, "edge      %s\n", edge)
	fmt.Fprintf(out, "process   %s\n", inspectProcess(machineDir))
	return nil
}

func clientVersion() string {
	if v := strings.TrimSpace(buildmeta.SoftVersion); v != "" {
		return v
	}
	return "-"
}

func inspectProcess(machineDir string) string {
	holder, err := runlock.Inspect(machineDir)
	if err != nil {
		return "unknown (" + err.Error() + ")"
	}
	if holder == nil {
		if reason := startupFailureForStatus(machineDir); reason != "" {
			return "not running (" + reason + ")"
		}
		return "not running"
	}
	mode := strings.TrimSpace(holder.Mode)
	if mode == "" {
		mode = "unknown"
	}
	return fmt.Sprintf("%s pid %d", mode, holder.PID)
}

func startupFailureForStatus(machineDir string) string {
	data, err := os.ReadFile(config.LogPath(machineDir))
	if err != nil || len(data) == 0 {
		return ""
	}
	const tail = 64 * 1024
	if len(data) > tail {
		data = data[len(data)-tail:]
	}
	return lastUnrecoveredStartupFailure(string(data))
}
