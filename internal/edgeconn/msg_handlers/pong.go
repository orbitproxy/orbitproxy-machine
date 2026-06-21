package msg_handlers

import "github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"

func (handler *Handler) HandlePong(m msg.Message) error {
	_, ok := m.(*msg.Pong)
	if !ok {
		return nil
	}
	handler.conn.MarkPong()
	logger := handler.conn.Logger()
	logger.Debug().
		Str("component", "edgeconn").
		Str("machine_id", handler.conn.MachineID()).
		Msg("receive heartbeat from edge")
	return nil
}
