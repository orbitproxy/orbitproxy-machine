package msg_handlers

import "github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"

func (handler *Handler) HandleReqWorkConn(m msg.Message) error {
	_ = m
	sessionID := handler.conn.SessionID()
	if sessionID == "" {
		logger := handler.conn.Logger()
		logger.Warn().
			Str("component", "edgeconn").
			Str("machine_id", handler.conn.MachineID()).
			Msg("req_work_conn before server hello, ignoring")
		return nil
	}
	return handler.conn.OpenWork(sessionID)
}
