package edgeconn

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/hashicorp/yamux"
	"github.com/orbitlink/orbitlink-machine/internal/buildmeta"
	"github.com/orbitlink/orbitlink-machine/internal/config"
	"github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"
	ecutils "github.com/orbitlink/orbitlink-machine/internal/edgeconn/utils"
	"github.com/rs/zerolog"
)

type connConfig struct {
	EdgeAddr      string
	TLSServerName string
	TLSCAPEM      string
	MachineID     string
	PrivateKeyPEM string
	SoftVersion   string
}

func connConfigFrom(cfg config.Config) connConfig {
	return connConfig{
		EdgeAddr:      cfg.EdgeAddr,
		TLSServerName: cfg.TLSServerName,
		TLSCAPEM:      cfg.TLSCAPEM,
		MachineID:     cfg.MachineID,
		PrivateKeyPEM: cfg.PrivateKeyPEM,
		SoftVersion:   buildmeta.SoftVersion,
	}
}

func buildTLSConfig(cfg connConfig) (*tls.Config, error) {
	serverName := cfg.TLSServerName
	if serverName == "" {
		host, _, err := net.SplitHostPort(cfg.EdgeAddr)
		if err != nil {
			return nil, fmt.Errorf("parse edge_addr %q: %w", cfg.EdgeAddr, err)
		}
		serverName = host
	}

	tlsConfig := &tls.Config{
		ServerName: serverName,
		MinVersion: tls.VersionTLS12,
	}

	if cfg.TLSCAPEM != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(cfg.TLSCAPEM)) {
			return nil, fmt.Errorf("tls_ca_pem is not a valid PEM certificate bundle")
		}
		tlsConfig.RootCAs = pool
	}

	return tlsConfig, nil
}

// DialControlSession 建立到 edge 的控制会话（TLS + yamux + ClientHello）。
func DialControlSession(ctx context.Context, cfg config.Config, logger zerolog.Logger) (*SessionContext, error) {
	connCfg := connConfigFrom(cfg)

	dialer := &net.Dialer{}
	rawConn, err := dialer.DialContext(ctx, "tcp", connCfg.EdgeAddr)
	if err != nil {
		return nil, fmt.Errorf("dial edge %s: %w", connCfg.EdgeAddr, err)
	}

	tlsConfig, err := buildTLSConfig(connCfg)
	if err != nil {
		_ = rawConn.Close()
		return nil, err
	}

	tlsConn := tls.Client(rawConn, tlsConfig)
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("tls handshake with edge: %w", err)
	}

	yamuxSession, err := yamux.Client(tlsConn, nil)
	if err != nil {
		_ = tlsConn.Close()
		return nil, fmt.Errorf("open yamux session: %w", err)
	}

	controlStream, err := yamuxSession.Open()
	if err != nil {
		_ = yamuxSession.Close()
		return nil, fmt.Errorf("open yamux control stream: %w", err)
	}

	success := false
	defer func() {
		if !success {
			_ = controlStream.Close()
			_ = yamuxSession.Close()
		}
	}()

	hello, err := ecutils.NewClientHello(connCfg.MachineID, connCfg.SoftVersion)
	if err != nil {
		return nil, err
	}
	hello, err = ecutils.SignClientHello(connCfg.PrivateKeyPEM, hello)
	if err != nil {
		return nil, err
	}
	if err := msg.WriteMsg(controlStream, hello); err != nil {
		return nil, fmt.Errorf("send client hello: %w", err)
	}

	logger.Info().
		Str("component", "edgeconn").
		Str("machine_id", connCfg.MachineID).
		Str("edge_addr", connCfg.EdgeAddr).
		Msg("edge connected, waiting for server hello")

	success = true
	return &SessionContext{
		ConnConfig:    connCfg,
		Yamux:         yamuxSession,
		ControlStream: controlStream,
	}, nil
}
