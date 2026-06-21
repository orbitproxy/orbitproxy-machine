package edgeconn

import (
	"time"

	"github.com/orbitlink/orbitlink-machine/internal/backoff"
	"github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"
)

const (
	heartbeatInterval = 30 * time.Second
	heartbeatTimeout  = 90 * time.Second
)

// heartbeatWorker sends heartbeat to edge and checks heartbeat timeout
// (对齐 frpc client.Control.heartbeatWorker).
func (ctl *Control) heartbeatWorker() {
	interval := heartbeatInterval
	timeout := heartbeatTimeout

	if interval > 0 {
		sendHeartBeat := func() (bool, error) {
			ctl.logger.Debug().
				Str("machine_id", ctl.sessionCtx.ConnConfig.MachineID).
				Msg("send heartbeat to edge")
			_ = ctl.msgDispatcher.Send(msg.Ping{Ts: time.Now().Unix()})
			return false, nil
		}

		go backoff.BackoffUntil(sendHeartBeat,
			backoff.NewFastBackoffManager(backoff.FastBackoffOptions{
				Duration:           interval,
				InitDurationIfFail: time.Second,
				Factor:             2.0,
				Jitter:             0.1,
				MaxDuration:        interval,
			}),
			true, ctl.doneCh,
		)
	}

	if interval > 0 && timeout > 0 {
		go backoff.Until(func() {
			if time.Since(ctl.lastPong.Load().(time.Time)) > timeout {
				ctl.logger.Warn().
					Str("machine_id", ctl.sessionCtx.ConnConfig.MachineID).
					Msg("heartbeat timeout")
				ctl.closeSession()
			}
		}, time.Second, ctl.doneCh)
	}
}
