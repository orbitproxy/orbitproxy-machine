// Package daemon 负责以当前用户拉起与停止后台 run 进程（mode=background）。
package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/runlock"
)

var (
	ErrNotRunning    = errors.New("orbitproxy is not running")
	ErrNotBackground = errors.New("orbitproxy is not running in background mode")
)

// OutputPath 是后台进程 stdout/stderr 的落点；与 slog 的滚动日志分开，避免轮转后句柄错位。
func OutputPath(machineDir string) string {
	return filepath.Join(config.LogsDir(machineDir), "background.out")
}

// Spawner 便于测试替换真实的进程拉起。
type Spawner func(executable string, args []string, outputPath string) (int, error)

// Options 描述一次 EnsureRunning。
type Options struct {
	Root       string
	MachineKey string
	MachineDir string
	Executable string
	Spawn      Spawner
}

// EnsureRunning 已有持锁者（任意模式）时直接返回它；否则拉起后台进程。
func EnsureRunning(opts Options) (holder *runlock.Holder, started bool, err error) {
	holder, err = runlock.Inspect(opts.MachineDir)
	if err != nil {
		return nil, false, err
	}
	if holder != nil {
		return holder, false, nil
	}
	executable := opts.Executable
	if executable == "" {
		executable, err = os.Executable()
		if err != nil {
			return nil, false, fmt.Errorf("resolve executable: %w", err)
		}
	}
	spawn := opts.Spawn
	if spawn == nil {
		spawn = spawnDetached
	}
	if err := os.MkdirAll(config.LogsDir(opts.MachineDir), 0o700); err != nil {
		return nil, false, err
	}
	args := []string{"--workdir", opts.Root, "run", "--machine-key", opts.MachineKey, "--log=legacy", "--background-child"}
	pid, err := spawn(executable, args, OutputPath(opts.MachineDir))
	if err != nil {
		return nil, false, fmt.Errorf("start background client: %w", err)
	}
	return &runlock.Holder{PID: pid, Mode: runlock.ModeBackground}, true, nil
}

// Stop 只结束 mode=background 的持锁进程，并等待锁释放。
func Stop(machineDir string, timeout time.Duration) (*runlock.Holder, error) {
	holder, err := runlock.Inspect(machineDir)
	if err != nil {
		return nil, err
	}
	if holder == nil {
		return nil, ErrNotRunning
	}
	if holder.Mode != runlock.ModeBackground {
		return holder, ErrNotBackground
	}
	if err := terminate(holder.PID); err != nil {
		return holder, fmt.Errorf("stop pid %d: %w", holder.PID, err)
	}
	// 锁释放后进程可能还在退出途中写文件，要等进程真正结束，logout 才能安全删除目录。
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		current, err := runlock.Inspect(machineDir)
		if err == nil && current == nil && !processAlive(holder.PID) {
			return holder, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := kill(holder.PID); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return holder, fmt.Errorf("kill pid %d: %w", holder.PID, err)
	}
	return holder, nil
}

func openOutput(outputPath string) (*os.File, error) {
	return os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

func startDetached(cmd *exec.Cmd, outputPath string) (int, error) {
	output, err := openOutput(outputPath)
	if err != nil {
		return 0, err
	}
	defer func() { _ = output.Close() }()
	cmd.Stdin = nil
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

func kill(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}
