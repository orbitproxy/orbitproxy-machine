package edgeconn

import (
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/hashicorp/yamux"
	"github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"
	"github.com/rs/zerolog"
)

func openWorkConn(logger zerolog.Logger, yamuxSession *yamux.Session, machineID, sessionID string) error {
	stream, err := yamuxSession.Open()
	if err != nil {
		return fmt.Errorf("open yamux work stream: %w", err)
	}

	newWork := msg.NewWorkConn{
		SessionID: sessionID,
		MachineID: machineID,
	}
	if err := msg.WriteMsg(stream, newWork); err != nil {
		_ = stream.Close()
		return fmt.Errorf("send new_work_conn: %w", err)
	}

	go relayWorkStream(logger, machineID, stream)
	return nil
}

func relayWorkStream(logger zerolog.Logger, machineID string, stream net.Conn) {
	defer stream.Close()

	m, err := msg.ReadMsg(stream)
	if err != nil {
		logger.Debug().
			Str("component", "edgeconn").
			Str("machine_id", machineID).
			Err(err).
			Msg("read start_work_conn failed")
		return
	}
	start, ok := m.(*msg.StartWorkConn)
	if !ok {
		logger.Debug().
			Str("component", "edgeconn").
			Str("machine_id", machineID).
			Str("type", string(m.MsgType())).
			Msg("work stream first message is not start_work_conn")
		return
	}
	if start.Error != "" {
		logger.Warn().
			Str("component", "edgeconn").
			Str("machine_id", machineID).
			Str("proxy_id", start.ProxyID).
			Str("error", start.Error).
			Msg("start_work_conn rejected by edge")
		return
	}

	localAddr := net.JoinHostPort(start.LocalIP, strconv.Itoa(start.LocalPort))
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		logger.Warn().
			Str("component", "edgeconn").
			Str("machine_id", machineID).
			Str("proxy_id", start.ProxyID).
			Str("local_addr", localAddr).
			Err(err).
			Msg("dial local backend failed")
		return
	}
	defer localConn.Close()

	logger.Debug().
		Str("component", "edgeconn").
		Str("machine_id", machineID).
		Str("proxy_id", start.ProxyID).
		Str("endpoint_id", start.EndpointID).
		Str("local_addr", localAddr).
		Msg("work conn relay started")

	joinConnections(stream, localConn)
}

func joinConnections(a, b io.ReadWriteCloser) {
	done := make(chan struct{}, 2)
	copyConn := func(dst io.Writer, src io.Reader) {
		_, _ = io.Copy(dst, src)
		done <- struct{}{}
	}
	go copyConn(a, b)
	go copyConn(b, a)
	<-done
}
