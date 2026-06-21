package edgeconn

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"
	"github.com/rs/zerolog"
)

// Control 表示一条到 edge 的控制连接（对齐 frpc client.Control）。
type Control struct {
	ctx        context.Context
	sessionCtx *SessionContext
	session    *sessionState
	logger     zerolog.Logger

	msgDispatcher *msg.Dispatcher

	doneCh chan struct{}
	once   sync.Once

	// lastPong 记录最近一次收到 Pong 的时间（对齐 frpc Control.lastPong）。
	lastPong atomic.Value // time.Time
}

func NewControl(ctx context.Context, sessionCtx *SessionContext, logger zerolog.Logger) *Control {
	ctl := &Control{
		ctx:           ctx,
		sessionCtx:    sessionCtx,
		session:       &sessionState{},
		logger:        logger.With().Str("component", "edgeconn").Logger(),
		msgDispatcher: msg.NewDispatcher(sessionCtx.ControlStream),
		doneCh:        make(chan struct{}),
	}
	ctl.lastPong.Store(time.Now())
	return ctl
}

func (ctl *Control) Run() {
	go ctl.worker()
}

func (ctl *Control) worker() {
	defer ctl.finish()

	ctl.registerMsgHandlers()

	go ctl.heartbeatWorker()
	go ctl.msgDispatcher.Run()

	<-ctl.msgDispatcher.Done()
	if ctl.ctx.Err() == nil {
		ctl.logger.Debug().Msg("control message dispatcher exited")
	}
	ctl.closeSession()
}

func (ctl *Control) Done() <-chan struct{} {
	return ctl.doneCh
}

func (ctl *Control) Close() {
	ctl.closeSession()
}

func (ctl *Control) finish() {
	ctl.once.Do(func() { close(ctl.doneCh) })
}

func (ctl *Control) closeSession() {
	ctl.sessionCtx.Close()
}
