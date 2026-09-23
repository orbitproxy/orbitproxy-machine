package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	orbitproxy "github.com/orbitproxy/orbitproxy-go"
	"github.com/orbitproxy/orbitproxy-go/service"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/console"
	"github.com/orbitproxy/orbitproxy-machine/logger"
	"github.com/orbitproxy/orbitproxy-machine/runlock"
	"github.com/spf13/cobra"
)

var (
	runMachineKeyFlag string
	runFromService    bool
	runLogModeFlag    string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the OrbitProxy machine client",
	Long:  "Load local machine config and run until interrupted.",
	RunE:  runMachine,
}

func init() {
	runCmd.Flags().StringVar(&runMachineKeyFlag, "machine-key", "", "machine key under --workdir (required when multiple machines exist)")
	runCmd.Flags().StringVar(&runLogModeFlag, "log", "", `stdout format: status card on a TTY (default), or "legacy" for slog`)
	runCmd.Flags().BoolVar(&runFromService, "from-service", false, "set when launched by the OS service")
	_ = runCmd.Flags().MarkHidden("from-service")
	rootCmd.AddCommand(runCmd)
}

func runMachine(cmd *cobra.Command, _ []string) error {
	if !runFromService {
		if err := refuseRootForeground(); err != nil {
			return err
		}
	}
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mode := runlock.ModeForeground
	if runFromService {
		mode = runlock.ModeService
	}
	return runMachineWithContext(ctx, workDirFlag, runMachineKeyFlag, mode, runLogModeFlag)
}

func acquireMachineLock(machineDir, mode string) (*runlock.Lock, error) {
	lock, err := runlock.Acquire(machineDir, mode)
	if err == nil {
		return lock, nil
	}
	var conflict *runlock.ConflictError
	if !errors.As(err, &conflict) {
		return nil, err
	}

	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		lock, err = runlock.Acquire(machineDir, mode)
		if err == nil {
			return lock, nil
		}
		if !errors.As(err, &conflict) {
			return nil, err
		}
	}
	return nil, err
}

func runMachineWithContext(ctx context.Context, workdir, machineKey, mode, logMode string) error {
	legacy, err := console.ParseLogMode(logMode)
	if err != nil {
		return err
	}

	root, err := config.ResolveRoot(workdir)
	if err != nil {
		return err
	}

	machineDir, err := config.ResolveMachineDir(root, machineKey)
	if err != nil {
		return err
	}

	machineCfg, err := config.Load(machineDir)
	if err != nil {
		return err
	}
	if err := config.ApplyMcpRuntimeEnv(machineDir); err != nil {
		noteServiceStartupFailure(machineDir, err)
		return err
	}

	lock, err := acquireMachineLock(machineDir, mode)
	if err != nil {
		noteServiceStartupFailure(machineDir, err)
		return err
	}
	defer func() { _ = lock.Close() }()

	useCard := console.UseCard(legacy, mode == runlock.ModeService, console.IsTerminal(os.Stdout))
	logsDir := config.LogsDir(machineCfg.WorkDir)
	slogLogger := logger.New(logger.Options{
		Level:  machineCfg.Logger.Level,
		LogDir: logsDir,
		Stdout: !useCard,
	})
	slogLogger.Info("orbitproxy started",
		"machine_key", machineCfg.MachineKey,
		"edge_addr", machineCfg.EdgeAddr,
		"soft_version", buildmeta.SoftVersion,
		"workdir", machineCfg.WorkDir,
		"root", root,
		"mode", mode,
	)

	card := &runCard{
		status: console.Status{
			Version:    buildmeta.SoftVersion,
			State:      "connecting",
			MachineKey: machineCfg.MachineKey,
			EdgeAddr:   machineCfg.EdgeAddr,
			LogsDir:    logsDir,
		},
	}
	if useCard {
		console.EnableVirtualTerminal(os.Stdout)
		card.ui = console.NewUI(os.Stdout, true)
		defer card.ui.Close()
		card.ui.Card(card.status)
	}

	svc, err := orbitproxy.Start(ctx, orbitproxy.Identity{
		MachineKey:    machineCfg.MachineKey,
		EdgeAddr:      machineCfg.EdgeAddr,
		MachineCACert: machineCfg.MachineCACert,
		PrivateKeyPEM: machineCfg.PrivateKeyPEM,
	}, orbitproxy.StartOptions{
		Logger:         orbitproxy.LoggerOptions{Slog: slogLogger},
		SoftVersion:    buildmeta.SoftVersion,
		DataRoot:       root,
		OnConnected:    card.connected,
		OnEndpoints:    card.endpoints,
		OnReconnecting: card.reconnecting,
		OnDisconnected: card.disconnected,
	})
	if err != nil {
		return err
	}
	defer svc.Close()

	select {
	case <-ctx.Done():
		if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case <-svc.Done():
		err := svc.Err()
		if err == nil || errors.Is(err, context.Canceled) {
			return nil
		}
		if strings.Contains(err.Error(), "permanently disconnected") {
			return nil
		}
		return err
	}
}

type runCard struct {
	mu     sync.Mutex
	ui     *console.UI
	status console.Status
}

func (c *runCard) connected(sessionID string) {
	if c.ui == nil {
		return
	}
	c.mu.Lock()
	c.status.State = "online"
	c.status.SessionID = sessionID
	snap := c.status
	c.mu.Unlock()
	c.ui.Card(snap)
}

func (c *runCard) endpoints(endpoints []service.EndpointStatus) {
	if c.ui == nil {
		return
	}
	c.mu.Lock()
	c.status.Forwards = endpointForwards(endpoints)
	snap := c.status
	c.mu.Unlock()
	c.ui.Card(snap)
}

func (c *runCard) reconnecting(attempt int, reason string) {
	if c.ui == nil {
		return
	}
	c.mu.Lock()
	c.status.State = "reconnecting"
	snap := c.status
	c.mu.Unlock()
	c.ui.Event(snap, fmt.Sprintf("reconnecting  attempt %d  %s", attempt, reason))
}

func (c *runCard) disconnected(reason string, permanent bool) {
	if c.ui == nil {
		return
	}
	c.mu.Lock()
	c.status.State = "offline"
	snap := c.status
	c.mu.Unlock()
	kind := "disconnected"
	if permanent {
		kind = "permanently disconnected"
	}
	c.ui.Event(snap, fmt.Sprintf("%s  %s", kind, reason))
}

func endpointForwards(endpoints []service.EndpointStatus) []console.Forward {
	out := make([]console.Forward, 0, len(endpoints))
	for _, e := range endpoints {
		kind := strings.TrimSpace(e.Type)
		if kind == "" {
			kind = strings.TrimSpace(e.Protocol)
		}
		local := strings.TrimSpace(e.LocalAddr)
		if local == "" && e.Delivery == service.DeliveryInProcess {
			local = "in-process"
		}
		out = append(out, console.Forward{
			Kind:       kind,
			Domain:     strings.TrimSpace(e.PubHost),
			EndpointID: strings.TrimSpace(e.EndpointID),
			Local:      local,
		})
	}
	return out
}
