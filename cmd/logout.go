package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/daemon"
	"github.com/orbitproxy/orbitproxy-machine/machineapi"
	"github.com/spf13/cobra"
)

var (
	logoutYesFlag        bool
	logoutMachineKeyFlag string
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out this computer",
	Long:  "Invalidate this computer's key on the control plane, stop the background client and remove local credentials. Tunnels, domains and the machine record are kept.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(false), func() error {
			return runLogout(cmd.Context(), env, logoutMachineKeyFlag, logoutYesFlag)
		})
	},
}

func init() {
	logoutCmd.Flags().BoolVar(&logoutYesFlag, "yes", false, "skip confirmation (implied without a TTY)")
	logoutCmd.Flags().StringVar(&logoutMachineKeyFlag, "machine-key", "", "machine key under --workdir")
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(ctx context.Context, env *cliEnv, machineKey string, yes bool) error {
	key, err := config.SelectMachineKey(env.root, machineKey)
	if err != nil {
		if errors.Is(err, config.ErrMachineAmbiguous) {
			return cliout.New("machine_ambiguous", err.Error())
		}
		if errors.Is(err, config.ErrNoMachine) {
			if err := config.RemoveIfExists(config.PendingLoginPath(env.root)); err != nil {
				return err
			}
			fmt.Fprintln(env.out, "not logged in")
			return nil
		}
		return err
	}
	machineDir, err := config.MachineDir(env.root, key)
	if err != nil {
		return err
	}

	if env.interactive && !yes {
		fmt.Fprintf(env.out, "登出 %s？本机将不能再分享，已有分享与域名保留。[y/N] ", key)
		answer, _ := bufio.NewReader(env.in).ReadString('\n')
		if answer != "y\n" && answer != "Y\n" && answer != "y\r\n" && answer != "Y\r\n" {
			fmt.Fprintln(env.out, "cancelled")
			return nil
		}
	}

	if cfg, err := config.Load(machineDir); err == nil {
		if client, err := env.machineClient(cfg, machineapi.LogoutTimeout); err == nil {
			remoteCtx, cancel := context.WithTimeout(ctx, machineapi.LogoutTimeout)
			if err := client.Logout(remoteCtx); err != nil {
				fmt.Fprintf(env.errOut, "warning: remote logout failed: %s\n", err)
			}
			cancel()
		}
	}

	if holder, err := daemon.Stop(machineDir, 5*time.Second); err != nil && !errors.Is(err, daemon.ErrNotRunning) {
		if errors.Is(err, daemon.ErrNotBackground) && holder != nil {
			fmt.Fprintf(env.errOut, "warning: orbitproxy is still running (%s pid %d); stop it manually\n", holder.Mode, holder.PID)
		} else {
			fmt.Fprintf(env.errOut, "warning: stop background client: %s\n", err)
		}
	}

	if err := config.WriteKeyFile(config.LastMachinePath(env.root), key); err != nil {
		return err
	}
	if err := os.RemoveAll(machineDir); err != nil {
		return fmt.Errorf("remove machine config %s: %w", machineDir, err)
	}
	if pointer, err := config.ReadKeyFile(config.DefaultPointerPath(env.root)); err == nil && pointer == key {
		if err := config.RemoveIfExists(config.DefaultPointerPath(env.root)); err != nil {
			return err
		}
	}
	if err := config.RemoveIfExists(config.PendingLoginPath(env.root)); err != nil {
		return err
	}
	fmt.Fprintf(env.out, "logged out: %s\n", key)
	return nil
}
