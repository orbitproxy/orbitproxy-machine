package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/orbitlink/orbitlink-machine/internal/backoff"
	"github.com/orbitlink/orbitlink-machine/internal/buildmeta"
	"github.com/orbitlink/orbitlink-machine/internal/config"
	"github.com/orbitlink/orbitlink-machine/internal/edgeconn"
	"github.com/orbitlink/orbitlink-machine/internal/logger"
	"github.com/rs/zerolog"
)

const (
	firstLoginMaxBackoff  = 10 * time.Second
	reconnectMaxBackoff   = 20 * time.Second
)

// Service is the machine client service that connects to edge (对齐 frpc client.Service).
type Service struct {
	cfg    *config.Config
	logger zerolog.Logger

	ctlMu sync.RWMutex
	ctl   *edgeconn.Control

	ctx    context.Context
	cancel context.CancelFunc
}

func New(cfg *config.Config) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	logger.Init(cfg.Logger.Level, cfg.WorkDir)

	return &Service{
		cfg:    cfg,
		logger: logger.MultiLogger(),
	}, nil
}

func (svr *Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	svr.ctx = ctx
	svr.cancel = cancel

	svr.logger.Info().
		Str("machine_id", svr.cfg.MachineID).
		Str("edge_addr", svr.cfg.EdgeAddr).
		Str("soft_version", buildmeta.SoftVersion).
		Str("workdir", svr.cfg.WorkDir).
		Msg("orbitlink-machine started")

	svr.loopLoginUntilSuccess(firstLoginMaxBackoff)
	if svr.getCtl() == nil {
		svr.stop()
		if err := svr.ctx.Err(); err != nil {
			return err
		}
		return fmt.Errorf("connect to edge failed")
	}

	go svr.keepControllerWorking()

	<-svr.ctx.Done()
	svr.stop()
	return nil
}

func (svr *Service) keepControllerWorking() {
	for {
		if svr.ctx.Err() != nil {
			return
		}

		ctl := svr.getCtl()
		if ctl == nil {
			return
		}

		select {
		case <-svr.ctx.Done():
			return
		case <-ctl.Done():
		}

		if svr.ctx.Err() != nil {
			return
		}

		svr.logger.Warn().
			Str("machine_id", svr.cfg.MachineID).
			Str("edge_addr", svr.cfg.EdgeAddr).
			Msg("control connection closed, reconnecting")

		svr.loopLoginUntilSuccess(reconnectMaxBackoff)
		if svr.getCtl() == nil && svr.ctx.Err() != nil {
			return
		}
	}
}

func (svr *Service) loopLoginUntilSuccess(maxInterval time.Duration) {
	policy := backoff.NewExponentialBackOff()
	policy.MaxInterval = maxInterval

	_ = backoff.Loop(svr.ctx, policy, func(ctx context.Context) error {
		if err := svr.loginOnce(ctx); err != nil {
			svr.logger.Warn().
				Str("machine_id", svr.cfg.MachineID).
				Str("edge_addr", svr.cfg.EdgeAddr).
				Err(err).
				Msg("connect to edge error")
			return err
		}

		backoff.Reset(policy)
		svr.logger.Info().
			Str("machine_id", svr.cfg.MachineID).
			Str("edge_addr", svr.cfg.EdgeAddr).
			Msg("login to edge success")
		return nil
	}, func(err error, wait time.Duration) {
		if svr.ctx.Err() != nil {
			return
		}
		svr.logger.Warn().
			Str("machine_id", svr.cfg.MachineID).
			Str("edge_addr", svr.cfg.EdgeAddr).
			Err(err).
			Dur("retry_in", wait).
			Msg("connect to edge failed, retrying")
	})
}

func (svr *Service) loginOnce(ctx context.Context) error {
	sessionCtx, err := edgeconn.DialControlSession(ctx, *svr.cfg, svr.logger)
	if err != nil {
		return err
	}

	ctl := edgeconn.NewControl(ctx, sessionCtx, svr.logger)
	ctl.Run()

	svr.ctlMu.Lock()
	if svr.ctl != nil {
		svr.ctl.Close()
	}
	svr.ctl = ctl
	svr.ctlMu.Unlock()
	return nil
}

func (svr *Service) getCtl() *edgeconn.Control {
	svr.ctlMu.RLock()
	defer svr.ctlMu.RUnlock()
	return svr.ctl
}

func (svr *Service) stop() {
	svr.ctlMu.Lock()
	defer svr.ctlMu.Unlock()
	if svr.ctl != nil {
		svr.ctl.Close()
		svr.ctl = nil
	}
}
