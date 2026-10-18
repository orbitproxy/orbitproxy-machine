package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/machineapi"
	"github.com/spf13/cobra"
)

var (
	closeJSONFlag       bool
	closeMachineKeyFlag string
)

var closeCmd = &cobra.Command{
	Use:   "close <proxyId|port>",
	Short: "Close a tunnel by proxy ID or local port",
	Long:  "Close a tunnel on this computer. A numeric argument matches the local port; the domain is kept for reuse.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(closeJSONFlag), func() error {
			return runClose(cmd.Context(), env, args[0], closeMachineKeyFlag, closeJSONFlag)
		})
	},
}

func init() {
	closeCmd.Flags().BoolVar(&closeJSONFlag, "json", false, "print JSON")
	closeCmd.Flags().StringVar(&closeMachineKeyFlag, "machine-key", "", "machine key under --workdir")
	rootCmd.AddCommand(closeCmd)
}

type closeData struct {
	ProxyID   string `json:"proxy_id"`
	PublicURL string `json:"public_url,omitempty"`
	LocalAddr string `json:"local_addr,omitempty"`
	Closed    bool   `json:"closed"`
}

func runClose(ctx context.Context, env *cliEnv, target, machineKey string, jsonOutput bool) error {
	current, err := env.currentSession(machineKey)
	if err != nil {
		return err
	}
	data := closeData{ProxyID: target}
	if port, ok := parsePort(target); ok {
		tunnel, err := tunnelByPort(ctx, current.client, port)
		if err != nil {
			return err
		}
		data = closeData{ProxyID: tunnel.ProxyID, PublicURL: tunnel.PublicURL, LocalAddr: tunnel.LocalAddr}
	}
	if err := current.client.DeleteTunnel(ctx, data.ProxyID); err != nil {
		return err
	}
	data.Closed = true
	return env.printer(jsonOutput).Success(data, func(out io.Writer) {
		fmt.Fprintf(out, "closed %s\n", data.ProxyID)
	})
}

func tunnelByPort(ctx context.Context, client *machineapi.Client, port int) (machineapi.Tunnel, error) {
	tunnels, err := client.ListTunnels(ctx)
	if err != nil {
		return machineapi.Tunnel{}, err
	}
	var matches []machineapi.Tunnel
	for _, tunnel := range tunnels {
		_, localPort, err := net.SplitHostPort(tunnel.LocalAddr)
		if err == nil && localPort == strconv.Itoa(port) {
			matches = append(matches, tunnel)
		}
	}
	switch len(matches) {
	case 0:
		return machineapi.Tunnel{}, cliout.New("tunnel_not_found", fmt.Sprintf("本机没有分享本地端口 %d", port))
	case 1:
		return matches[0], nil
	}
	cliErr := cliout.New("ambiguous_port", fmt.Sprintf("本地端口 %d 对应多个分享，请用 proxy_id 关闭", port))
	cliErr.Data = map[string]any{"candidates": matches}
	return machineapi.Tunnel{}, cliErr
}
