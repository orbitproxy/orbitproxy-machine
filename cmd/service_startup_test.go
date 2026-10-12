package main

import (
	"strings"
	"testing"
)

func TestLastUnrecoveredStartupFailureIgnoresRecoveredStart(t *testing.T) {
	chunk := strings.Join([]string{
		`time=1 level=ERROR msg="service startup failed" err="open mcp-runtime.env: permission denied"`,
		`time=2 level=INFO msg="orbitproxy started"`,
	}, "\n")
	if got := lastUnrecoveredStartupFailure(chunk); got != "" {
		t.Fatalf("recovered start should hide the old failure, got %q", got)
	}
	only := `time=1 level=ERROR msg="service startup failed" err="open mcp-runtime.env: permission denied"`
	if got := lastUnrecoveredStartupFailure(only); got != "open mcp-runtime.env: permission denied" {
		t.Fatalf("got %q", got)
	}
}
