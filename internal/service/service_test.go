package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/orbitlink/orbitlink-machine/internal/config"
	"github.com/orbitlink/orbitlink-machine/internal/edgeconn"
	"github.com/rs/zerolog"
)

func TestNewInitializesService(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t, testPrivateKeyPEM(t))
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if svc == nil {
		t.Fatal("New returned nil service")
	}
}

func TestStopClosesActiveControl(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	defer server.Close()

	ctl := edgeconn.NewControl(context.Background(), &edgeconn.SessionContext{ControlStream: client}, zerolog.Nop())
	ctl.Run()

	cfg := testConfig(t, testPrivateKeyPEM(t))
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	svc.ctl = ctl

	svc.stop()

	select {
	case <-ctl.Done():
	case <-time.After(time.Second):
		t.Fatal("control was not closed by stop")
	}
	if svc.ctl != nil {
		t.Fatal("control was not cleared after stop")
	}
}

func testPrivateKeyPEM(t *testing.T) string {
	t.Helper()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}

	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey returned error: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))
}

func testConfig(t *testing.T, privateKeyPEM string) *config.Config {
	t.Helper()
	return &config.Config{
		MachineID:     "mach_test",
		EdgeAddr:      "127.0.0.1:65535",
		PrivateKeyPEM: privateKeyPEM,
		WorkDir:       t.TempDir(),
		Logger:        config.LoggerConfig{Level: "error"},
	}
}
