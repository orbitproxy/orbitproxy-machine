package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/machineapi"
	"github.com/spf13/cobra"
)

const (
	defaultShareHost      = "127.0.0.1"
	proxyTypeAPIGateway   = "api_gateway"
	warnLocalNotListening = "local_not_listening"
)

var (
	proxyAddTypeFlag      string
	proxyAddProtocolFlag  string
	proxyAddPortFlag      string
	proxyAddHostFlag      string
	proxyAddSubdomainFlag string
	proxyAddJSONFlag      bool
	proxyAddMachineKey    string
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Manage proxies on this computer",
}

var proxyAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Create a proxy on this computer",
	Long:  "Create an API gateway proxy and print its public address. Does not start the client; run `orbitproxy run --background` after adding.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(proxyAddJSONFlag), func() error {
			return runProxyAdd(cmd.Context(), env, proxyAddOptions{
				proxyType:  proxyAddTypeFlag,
				protocol:   proxyAddProtocolFlag,
				port:       proxyAddPortFlag,
				host:       proxyAddHostFlag,
				subdomain:  proxyAddSubdomainFlag,
				machineKey: proxyAddMachineKey,
				json:       proxyAddJSONFlag,
			})
		})
	},
}

func init() {
	proxyAddCmd.Flags().StringVar(&proxyAddTypeFlag, "type", "", "proxy type (required; api_gateway)")
	proxyAddCmd.Flags().StringVar(&proxyAddProtocolFlag, "protocol", "", "public protocol for api_gateway: https or tcp")
	proxyAddCmd.Flags().StringVar(&proxyAddPortFlag, "port", "", "local port to forward")
	proxyAddCmd.Flags().StringVar(&proxyAddHostFlag, "host", defaultShareHost, "local host to forward to")
	proxyAddCmd.Flags().StringVar(&proxyAddSubdomainFlag, "subdomain", "", "https only: use or create this subdomain")
	proxyAddCmd.Flags().BoolVar(&proxyAddJSONFlag, "json", false, "print JSON")
	proxyAddCmd.Flags().StringVar(&proxyAddMachineKey, "machine-key", "", "machine key under --workdir")
	proxyCmd.AddCommand(proxyAddCmd)
	rootCmd.AddCommand(proxyCmd)
}

type proxyAddOptions struct {
	proxyType  string
	protocol   string
	port       string
	host       string
	subdomain  string
	machineKey string
	json       bool
}

type proxyAddData struct {
	ProxyID   string   `json:"proxy_id"`
	PublicURL string   `json:"public_url"`
	LocalAddr string   `json:"local_addr"`
	Protocol  string   `json:"protocol"`
	Created   bool     `json:"created"`
	Warnings  []string `json:"warnings"`
}

func runProxyAdd(ctx context.Context, env *cliEnv, opts proxyAddOptions) error {
	if opts.proxyType == "" {
		return cliout.New("invalid_request", "必须指定 --type")
	}
	if opts.proxyType != proxyTypeAPIGateway {
		return cliout.New("unsupported_proxy_type", fmt.Sprintf("暂不支持 --type=%s", opts.proxyType))
	}
	if opts.protocol != "https" && opts.protocol != "tcp" {
		return cliout.New("invalid_tunnel_protocol", "api_gateway 的 --protocol 只能是 https 或 tcp")
	}
	if opts.protocol == "tcp" && opts.subdomain != "" {
		return cliout.New("invalid_request", "--subdomain 只适用于 --protocol=https")
	}
	port, ok := parsePort(opts.port)
	if !ok {
		return cliout.New("invalid_port", fmt.Sprintf("端口必须是 1-65535 的整数，收到 %q", opts.port))
	}
	current, err := env.currentSession(opts.machineKey)
	if err != nil {
		return err
	}

	warnings := []string{}
	if !localListening(opts.host, port) {
		warnings = append(warnings, warnLocalNotListening)
	}
	created, err := current.client.CreateTunnel(ctx, machineapi.CreateTunnelRequest{
		Protocol:  opts.protocol,
		LocalPort: port,
		LocalHost: opts.host,
		Subdomain: opts.subdomain,
	})
	if err != nil {
		return err
	}
	data := proxyAddData{
		ProxyID:   created.ProxyID,
		PublicURL: created.PublicURL,
		LocalAddr: created.LocalAddr,
		Protocol:  created.Protocol,
		Created:   created.Created,
		Warnings:  warnings,
	}
	return env.printer(opts.json).Success(data, func(out io.Writer) {
		fmt.Fprintln(out, data.PublicURL)
		fmt.Fprintf(out, "%s -> %s (%s)\n", data.PublicURL, data.LocalAddr, data.ProxyID)
		for _, warning := range warnings {
			fmt.Fprintf(out, "warning: %s\n", warningText(warning))
		}
	})
}

func warningText(code string) string {
	if code == warnLocalNotListening {
		return "本地服务似乎没有启动（" + code + "）"
	}
	return code
}

func parsePort(raw string) (int, bool) {
	if raw == "" || len(raw) > 5 {
		return 0, false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != raw {
		return 0, false
	}
	return port, true
}

func localListening(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
