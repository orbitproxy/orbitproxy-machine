package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/runlock"
	"github.com/spf13/cobra"
)

var (
	statusJSONFlag       bool
	statusMachineKeyFlag string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show login state and local machine config",
	Long:  "First line is `logged_in yes|no`. Completes a pending `orbitproxy login` when the browser confirmation is done. Does not connect or take the run lock.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(statusJSONFlag), func() error {
			return runStatus(cmd.Context(), env, statusMachineKeyFlag, statusJSONFlag)
		})
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSONFlag, "json", false, "print JSON")
	statusCmd.Flags().StringVar(&statusMachineKeyFlag, "machine-key", "", "machine key under --workdir")
	rootCmd.AddCommand(statusCmd)
}

type statusData struct {
	LoggedIn   bool   `json:"logged_in"`
	MachineKey string `json:"machine_key,omitempty"`
	Running    bool   `json:"running"`
	Mode       string `json:"mode,omitempty"`
}

func runStatus(ctx context.Context, env *cliEnv, machineKey string, jsonOutput bool) error {
	printer := env.printer(jsonOutput)
	current, err := env.currentSession(machineKey)
	if err != nil {
		var cliErr *cliout.Error
		// 多台机器且无 default 指针时也视为未登录，login 完成后写入 default 即可消歧。
		if !errors.As(err, &cliErr) || machineKey != "" ||
			(cliErr.Code != "not_logged_in" && cliErr.Code != "machine_ambiguous") {
			return err
		}
		result, pendingErr := completePendingLogin(ctx, env)
		if result.machineKey == "" {
			return reportLoggedOut(printer, pendingErr)
		}
		current, err = env.currentSession(result.machineKey)
		if err != nil {
			return err
		}
	} else if err := config.RemoveIfExists(config.PendingLoginPath(env.root)); err != nil {
		return err
	}

	data := statusData{LoggedIn: true, MachineKey: current.machineKey}
	if holder, err := runlock.Inspect(current.machineDir); err == nil && holder != nil {
		data.Running = true
		data.Mode = holder.Mode
	}
	return printer.Success(data, func(out io.Writer) {
		fmt.Fprintln(out, "logged_in yes")
		if err := printLocalStatus(out, env.root); err != nil {
			fmt.Fprintf(out, "error     %s\n", err)
		}
	})
}

// reportLoggedOut 输出 logged_in no 并以退出码 3 结束；登录被拒绝或过期时附带原因。
func reportLoggedOut(printer cliout.Printer, cause error) error {
	data := statusData{LoggedIn: false}
	if err := printer.Success(data, func(out io.Writer) {
		fmt.Fprintln(out, "logged_in no")
		if cause != nil {
			fmt.Fprintf(out, "reason    %s\n", cliout.FromError(cause).Message)
		}
	}); err != nil {
		return err
	}
	return exitWith(cliout.ExitNotLoggedIn)
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
