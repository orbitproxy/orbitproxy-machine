package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/console"
	"github.com/orbitproxy/orbitproxy-machine/daemon"
	"github.com/orbitproxy/orbitproxy-machine/machineapi"
	"github.com/spf13/cobra"
)

// cliEnv 收拢 WorkBuddy 相关命令的外部依赖，测试里整体替换。
type cliEnv struct {
	root        string
	out         io.Writer
	errOut      io.Writer
	in          io.Reader
	interactive bool
	baseURL     string
	http        *http.Client
	now         func() time.Time
	sleep       func(time.Duration)
	hostname    func() (string, error)
	spawn       daemon.Spawner
	executable  string
	// 轮询上限，测试里缩短。
	onlineWait   time.Duration
	endpointWait time.Duration
	pollEvery    time.Duration
}

func newCLIEnv(cmd *cobra.Command) (*cliEnv, error) {
	root, err := config.ResolveRoot(workDirFlag)
	if err != nil {
		return nil, err
	}
	return &cliEnv{
		root:         root,
		out:          cmd.OutOrStdout(),
		errOut:       cmd.ErrOrStderr(),
		in:           cmd.InOrStdin(),
		interactive:  console.IsTerminal(os.Stdout) && console.IsTerminal(os.Stdin),
		baseURL:      buildmeta.ControlPlaneURL,
		http:         &http.Client{Timeout: machineapi.DefaultTimeout},
		now:          time.Now,
		sleep:        time.Sleep,
		hostname:     os.Hostname,
		onlineWait:   15 * time.Second,
		endpointWait: 15 * time.Second,
		pollEvery:    time.Second,
	}, nil
}

func (env *cliEnv) printer(jsonOutput bool) cliout.Printer {
	return cliout.Printer{JSON: jsonOutput, Out: env.out, Err: env.errOut}
}

// session 是已登录机器的本地身份与签名客户端。
type session struct {
	machineKey string
	machineDir string
	cfg        *config.Config
	client     *machineapi.Client
}

func (env *cliEnv) machineClient(cfg *config.Config, timeout time.Duration) (*machineapi.Client, error) {
	client, err := machineapi.NewClient(env.baseURL, cfg.MachineKey, cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	client.HTTP = &http.Client{Timeout: timeout, Transport: env.http.Transport}
	client.Now = env.now
	return client, nil
}

// currentSession 按 --machine-key > default > 唯一一台选机器；未登录与歧义返回对应 CLI 错误。
func (env *cliEnv) currentSession(explicitKey string) (*session, error) {
	key, err := config.SelectMachineKey(env.root, explicitKey)
	if err != nil {
		switch {
		case errors.Is(err, config.ErrMachineAmbiguous):
			return nil, cliout.New("machine_ambiguous", err.Error())
		case errors.Is(err, config.ErrNoMachine):
			return nil, cliout.New("not_logged_in", "本机还没有登录 OrbitProxy")
		}
		return nil, err
	}
	machineDir, err := config.MachineDir(env.root, key)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(machineDir)
	if err != nil {
		return nil, cliout.New("not_logged_in", err.Error())
	}
	client, err := env.machineClient(cfg, machineapi.DefaultTimeout)
	if err != nil {
		return nil, cliout.New("not_logged_in", err.Error())
	}
	return &session{machineKey: key, machineDir: machineDir, cfg: cfg, client: client}, nil
}

// runCLI 把错误统一输出成 cliout 格式并转换为退出码。
func runCLI(printer cliout.Printer, run func() error) error {
	if err := run(); err != nil {
		if _, ok := exitCodeOf(err); ok {
			return err
		}
		return exitWith(printer.Fail(err))
	}
	return nil
}
