package main

import (
	"context"
	"fmt"
	"io"

	"github.com/orbitproxy/orbitproxy-machine/machineapi"
	"github.com/spf13/cobra"
)

var (
	listJSONFlag       bool
	listMachineKeyFlag string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tunnels that target this computer",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(listJSONFlag), func() error {
			return runList(cmd.Context(), env, listMachineKeyFlag, listJSONFlag)
		})
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSONFlag, "json", false, "print JSON")
	listCmd.Flags().StringVar(&listMachineKeyFlag, "machine-key", "", "machine key under --workdir")
	rootCmd.AddCommand(listCmd)
}

type listData struct {
	Tunnels []machineapi.Tunnel `json:"tunnels"`
}

func runList(ctx context.Context, env *cliEnv, machineKey string, jsonOutput bool) error {
	current, err := env.currentSession(machineKey)
	if err != nil {
		return err
	}
	tunnels, err := current.client.ListTunnels(ctx)
	if err != nil {
		return err
	}
	if tunnels == nil {
		tunnels = []machineapi.Tunnel{}
	}
	return env.printer(jsonOutput).Success(listData{Tunnels: tunnels}, func(out io.Writer) {
		if len(tunnels) == 0 {
			fmt.Fprintln(out, "no tunnels")
			return
		}
		for _, tunnel := range tunnels {
			fmt.Fprintf(out, "%s  %s -> %s  %s\n", tunnel.ProxyID, tunnel.PublicURL, tunnel.LocalAddr, tunnel.Status)
		}
	})
}
