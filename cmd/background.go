package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/daemon"
	"github.com/orbitproxy/orbitproxy-machine/machineapi"
	"github.com/spf13/cobra"
)

// startBackground 让 run --background 自己脱离终端：父进程拉起子进程后等到连上 edge 再退出。
func startBackground(cmd *cobra.Command) error {
	env, err := newCLIEnv(cmd)
	if err != nil {
		return err
	}
	return runInBackground(cmd.Context(), env)
}

func runInBackground(ctx context.Context, env *cliEnv) error {
	return runCLI(env.printer(false), func() error {
		current, err := env.currentSession(runMachineKeyFlag)
		if err != nil {
			return err
		}
		holder, started, err := daemon.EnsureRunning(daemon.Options{
			Root:       env.root,
			MachineKey: current.machineKey,
			MachineDir: current.machineDir,
			Executable: env.executable,
			Spawn:      env.spawn,
		})
		if err != nil {
			return err
		}
		if started {
			fmt.Fprintf(env.out, "started background pid %d\n", holder.PID)
		} else if holder != nil {
			fmt.Fprintf(env.out, "already running (%s pid %d)\n", holder.Mode, holder.PID)
		}
		return waitOnline(ctx, env, current.client)
	})
}

// waitOnline 轮询 self 直到 edge_connected；控制面业务错误立即返回，网络错误持续重试。
func waitOnline(ctx context.Context, env *cliEnv, client *machineapi.Client) error {
	deadline := env.now().Add(env.onlineWait)
	var lastNetErr error
	for {
		self, err := client.Self(ctx)
		if err == nil && self.EdgeConnected {
			return nil
		}
		if err != nil {
			var netErr *machineapi.NetworkError
			if !errors.As(err, &netErr) {
				return err
			}
			lastNetErr = err
		} else {
			lastNetErr = nil
		}
		if !env.now().Before(deadline) {
			if lastNetErr != nil {
				return lastNetErr
			}
			return cliout.New("client_offline", "本机客户端在 15 秒内没有连上 orbitproxy")
		}
		env.sleep(env.pollEvery)
	}
}
