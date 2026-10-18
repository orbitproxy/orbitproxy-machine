package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/daemon"
	"github.com/spf13/cobra"
)

var stopMachineKeyFlag string

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the client started by `orbitproxy run --background`",
	Long:  "Stop the background client (mode=background). Service and foreground runs are not affected.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(false), func() error { return runStop(env, stopMachineKeyFlag) })
	},
}

func init() {
	stopCmd.Flags().StringVar(&stopMachineKeyFlag, "machine-key", "", "machine key under --workdir")
	rootCmd.AddCommand(stopCmd)
}

func runStop(env *cliEnv, machineKey string) error {
	key, err := config.SelectMachineKey(env.root, machineKey)
	if err != nil {
		if errors.Is(err, config.ErrMachineAmbiguous) {
			return cliout.New("machine_ambiguous", err.Error())
		}
		if errors.Is(err, config.ErrNoMachine) {
			return cliout.New("not_logged_in", "本机还没有登录 OrbitProxy")
		}
		return err
	}
	machineDir, err := config.MachineDir(env.root, key)
	if err != nil {
		return err
	}
	holder, err := daemon.Stop(machineDir, 5*time.Second)
	switch {
	case errors.Is(err, daemon.ErrNotRunning):
		fmt.Fprintln(env.out, "not running")
		return nil
	case errors.Is(err, daemon.ErrNotBackground):
		return cliout.New("not_background", fmt.Sprintf("OrbitProxy 正以 %s 模式运行（pid %d），stop 只停止后台进程", holder.Mode, holder.PID))
	case err != nil:
		return err
	}
	fmt.Fprintf(env.out, "stopped pid %d\n", holder.PID)
	return nil
}
