package edgeconn

import (
	"net"
	"sync"

	"github.com/hashicorp/yamux"
)

// SessionContext 是一次 edge 控制连接的会话上下文（对齐 frpc SessionContext）。
type SessionContext struct {
	ConnConfig    connConfig
	Yamux         *yamux.Session
	ControlStream net.Conn
}

func (session *SessionContext) Close() {
	if session.ControlStream != nil {
		_ = session.ControlStream.Close()
	}
	if session.Yamux != nil {
		_ = session.Yamux.Close()
	}
}

type sessionState struct {
	mu        sync.RWMutex
	edgeID    string
	sessionID string
}

func (state *sessionState) SetSession(edgeID, sessionID string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.edgeID = edgeID
	state.sessionID = sessionID
}

func (state *sessionState) SessionID() string {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.sessionID
}
