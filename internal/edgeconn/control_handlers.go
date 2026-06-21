package edgeconn

import (
	"time"

	"github.com/hashicorp/yamux"
	"github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"
	msghandler "github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg_handlers"
	"github.com/rs/zerolog"
)

type controlHandlerConn struct {
	ctl     *Control
	cfg     connConfig
	session *sessionState
	yamux   *yamux.Session
	logger  zerolog.Logger
}

func (conn *controlHandlerConn) MachineID() string {
	return conn.cfg.MachineID
}

func (conn *controlHandlerConn) SetSession(edgeID, sessionID string) {
	conn.session.SetSession(edgeID, sessionID)
}

func (conn *controlHandlerConn) SessionID() string {
	return conn.session.SessionID()
}

func (conn *controlHandlerConn) MarkPong() {
	conn.ctl.lastPong.Store(time.Now())
}

func (conn *controlHandlerConn) OpenWork(sessionID string) error {
	return openWorkConn(conn.logger, conn.yamux, conn.cfg.MachineID, sessionID)
}

func (conn *controlHandlerConn) Logger() zerolog.Logger {
	return conn.logger
}

func (ctl *Control) registerMsgHandlers() {
	conn := &controlHandlerConn{
		ctl:     ctl,
		cfg:     ctl.sessionCtx.ConnConfig,
		session: ctl.session,
		yamux:   ctl.sessionCtx.Yamux,
		logger:  ctl.logger,
	}
	messageHandler := msghandler.New(conn)

	ctl.msgDispatcher.RegisterHandler(&msg.ServerHello{}, ctl.wrapHandler(messageHandler.HandleServerHello))
	ctl.msgDispatcher.RegisterHandler(&msg.Pong{}, ctl.wrapHandler(messageHandler.HandlePong))
	ctl.msgDispatcher.RegisterHandler(&msg.ReqWorkConn{}, msg.AsyncHandler(ctl.wrapHandler(messageHandler.HandleReqWorkConn)))
	ctl.msgDispatcher.SetDefaultHandler(func(m msg.Message) {
		ctl.logger.Debug().
			Str("type", string(m.MsgType())).
			Msg("ignored edge control message")
	})
}

func (ctl *Control) wrapHandler(handler func(msg.Message) error) func(msg.Message) {
	return func(m msg.Message) {
		if err := handler(m); err != nil {
			ctl.logger.Warn().
				Str("machine_id", ctl.sessionCtx.ConnConfig.MachineID).
				Str("type", string(m.MsgType())).
				Err(err).
				Msg("handle edge msg failed, skipping")
		}
	}
}
