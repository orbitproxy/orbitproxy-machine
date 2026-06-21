package msg_handlers

import "github.com/rs/zerolog"

type Conn interface {
	MachineID() string
	SetSession(edgeID, sessionID string)
	SessionID() string
	MarkPong()
	OpenWork(sessionID string) error
	Logger() zerolog.Logger
}

type Handler struct {
	conn Conn
}

func New(conn Conn) *Handler {
	return &Handler{conn: conn}
}
