package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/machineapi"
	"github.com/spf13/cobra"
)

// 非 TTY 下 login 必须在 WorkBuddy auth 的 10 秒内返回。
const deviceStartTimeout = 8 * time.Second

var loginClient string

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in this computer through the browser",
	Long:  "Start a browser confirmation and register this computer. Without a TTY it prints the URL and exits; `orbitproxy status` completes the login.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		env, err := newCLIEnv(cmd)
		if err != nil {
			return err
		}
		return runCLI(env.printer(false), func() error { return runLogin(cmd.Context(), env) })
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginClient, "client", "", "product opening this login: cursor, copilot, claude, trae, or workbuddy")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(ctx context.Context, env *cliEnv) error {
	if current, err := env.currentSession(""); err == nil {
		fmt.Fprintf(env.out, "already logged in: %s\n", current.machineKey)
		return nil
	}

	publicKeyPEM, privateKeyPEM, err := generateKeyPair()
	if err != nil {
		return err
	}
	restoreKey, err := config.ReadKeyFile(config.LastMachinePath(env.root))
	if err != nil {
		return err
	}
	hostname, _ := env.hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	startCtx, cancel := context.WithTimeout(ctx, deviceStartTimeout)
	defer cancel()
	started, err := machineapi.StartDevice(startCtx, env.http, env.baseURL, machineapi.DeviceStartRequest{
		PublicKey:          publicKeyPEM,
		PublicKeyAlgorithm: publicKeyAlgorithm,
		Hostname:           hostname,
		MachineName:        hostname,
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
		ClientVersion:      buildmeta.SoftVersion,
		RestoreMachineKey:  restoreKey,
		Client:             loginClient,
	})
	if err != nil {
		return err
	}

	pending := config.PendingLogin{
		DeviceCode:              started.DeviceCode,
		UserCode:                started.UserCode,
		VerificationURIComplete: started.VerificationURIComplete,
		PrivateKeyPEM:           privateKeyPEM,
		PublicKeyPEM:            publicKeyPEM,
		ExpiresAt:               env.now().Add(time.Duration(started.ExpiresIn) * time.Second).UTC(),
		Interval:                started.Interval,
	}
	if err := config.SavePendingLogin(env.root, pending); err != nil {
		return err
	}

	fmt.Fprintln(env.out, started.VerificationURIComplete)
	fmt.Fprintf(env.out, "在浏览器中确认登录，验证码 %s\n", started.UserCode)
	// WorkBuddy 没有 TTY，它自己从第一行取出 URL 再打开浏览器。这里再开一次会变成两个窗口。
	if !env.interactive {
		return nil
	}
	if err := openBrowser(started.VerificationURIComplete); err != nil {
		fmt.Fprintf(env.errOut, "warning: 无法打开浏览器: %s\n", err)
	}
	return waitForLogin(ctx, env, pending.Interval)
}

// waitForLogin 仅在终端交互时使用：按 interval 轮询直到完成、拒绝或过期。
func waitForLogin(ctx context.Context, env *cliEnv, interval int) error {
	if interval <= 0 {
		interval = 5
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		env.sleep(time.Duration(interval) * time.Second)
		result, err := completePendingLogin(ctx, env)
		switch {
		case result.machineKey != "":
			fmt.Fprintf(env.out, "logged in: %s\n", result.machineKey)
			return nil
		case result.slowDown:
			interval += 5
		case result.pending:
		case err != nil:
			return err
		default:
			return cliout.New("login_expired", "登录已过期，请重新执行 orbitproxy login")
		}
	}
}

type pendingResult struct {
	machineKey string
	pending    bool
	slowDown   bool
}

// completePendingLogin 用 pending-login.json 取回一次授权结果；成功即落盘机器配置。
func completePendingLogin(ctx context.Context, env *cliEnv) (pendingResult, error) {
	pending, err := config.LoadPendingLogin(env.root)
	if err != nil || pending == nil {
		return pendingResult{}, err
	}
	if !env.now().Before(pending.ExpiresAt) {
		return pendingResult{}, config.RemoveIfExists(config.PendingLoginPath(env.root))
	}
	privateKey, err := machineapi.ParsePrivateKeyPEM(pending.PrivateKeyPEM)
	if err != nil {
		_ = config.RemoveIfExists(config.PendingLoginPath(env.root))
		return pendingResult{}, err
	}

	pollCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	raw, err := machineapi.PollDeviceToken(pollCtx, env.http, env.baseURL, pending.DeviceCode, privateKey, env.now())
	if err != nil {
		var apiErr *machineapi.APIError
		if !errors.As(err, &apiErr) {
			return pendingResult{pending: true}, err
		}
		switch apiErr.Code {
		case "authorization_pending":
			return pendingResult{pending: true}, nil
		case "slow_down":
			return pendingResult{pending: true, slowDown: true}, nil
		case "access_denied":
			_ = config.RemoveIfExists(config.PendingLoginPath(env.root))
			return pendingResult{}, cliout.New("login_denied", "登录已在浏览器中被拒绝")
		}
		_ = config.RemoveIfExists(config.PendingLoginPath(env.root))
		return pendingResult{}, err
	}

	var data registerResponseData
	if err := json.Unmarshal(raw, &data); err != nil {
		return pendingResult{}, fmt.Errorf("decode login result: %w", err)
	}
	if data.Machine.MachineKey == "" || data.Edge.Addr == "" || data.Edge.CACert == "" {
		return pendingResult{}, errors.New("control-plane login result is missing machine registration data")
	}
	machineDir, err := config.MachineDir(env.root, data.Machine.MachineKey)
	if err != nil {
		return pendingResult{}, err
	}
	machineType := data.Machine.Type
	if machineType == "" {
		machineType = "cli"
	}
	if err := config.Save(machineDir, config.Config{
		MachineKey:    data.Machine.MachineKey,
		Type:          machineType,
		EdgeAddr:      data.Edge.Addr,
		MachineCACert: data.Edge.CACert,
		PublicKeyPEM:  pending.PublicKeyPEM,
		PrivateKeyPEM: pending.PrivateKeyPEM,
	}); err != nil {
		return pendingResult{}, err
	}
	if err := config.WriteKeyFile(config.DefaultPointerPath(env.root), data.Machine.MachineKey); err != nil {
		return pendingResult{}, err
	}
	_ = config.RemoveIfExists(config.PendingLoginPath(env.root))
	_ = config.RemoveIfExists(config.LastMachinePath(env.root))
	return pendingResult{machineKey: data.Machine.MachineKey}, nil
}
