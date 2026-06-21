package msg_handlers

import (
	"fmt"

	"github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"
)

func (handler *Handler) HandleServerHello(m msg.Message) error {
	hello, ok := m.(*msg.ServerHello)
	if !ok {
		return fmt.Errorf("unexpected server hello type %T", m)
	}
	handler.conn.SetSession(hello.EdgeID, hello.SessionID)
	logger := handler.conn.Logger()
	logger.Info().
		Str("component", "edgeconn").
		Str("machine_id", handler.conn.MachineID()).
		Str("edge_id", hello.EdgeID).
		Str("session_id", hello.SessionID).
		Msg("edge session established")
	return nil
}
