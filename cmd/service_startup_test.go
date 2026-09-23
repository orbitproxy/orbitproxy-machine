package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseServiceConnectLogSuccessWins(t *testing.T) {
	chunk := strings.Join([]string{
		`time=2026-09-03T00:16:01.268+08:00 level=WARN msg="connect to edge error" err="edge rejected session: 客户端当前不可连接"`,
		`time=2026-09-03T00:16:08.200+08:00 level=INFO msg="login to edge success" machine_key=ck_x edge_addr=edge:6008`,
	}, "\n")
	got := parseServiceConnectLog(chunk)
	if got.success != "login to edge success" || got.fail != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseServiceConnectLogReject(t *testing.T) {
	chunk := `time=2026-09-03T00:16:01.268+08:00 level=WARN msg="connect to edge error" machine_key=ck_x err="edge rejected session: 客户端当前不可连接"`
	got := parseServiceConnectLog(chunk)
	if got.fail != "edge rejected session: 客户端当前不可连接" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseServiceConnectLogEmpty(t *testing.T) {
	got := parseServiceConnectLog("")
	if got.success != "" || got.fail != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseServiceConnectLogStartupFailure(t *testing.T) {
	chunk := `time=2026-09-22T22:30:44+08:00 level=ERROR msg="service startup failed" err="open /tmp/mcp-runtime.env: permission denied"`
	got := parseServiceConnectLog(chunk)
	if got.fail != "open /tmp/mcp-runtime.env: permission denied" || got.success != "" {
		t.Fatalf("got %+v", got)
	}
}

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

func TestInspectServiceLogSinceOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "orbitproxy.log")
	old := `time=1 level=WARN msg="connect to edge error" err="old reject"` + "\n"
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	offset := int64(len(old))
	if err := os.WriteFile(path, []byte(old+`time=2 level=INFO msg="login to edge success"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := inspectServiceLogSince(path, offset)
	if err != nil {
		t.Fatal(err)
	}
	if got.success != "login to edge success" {
		t.Fatalf("got %+v", got)
	}
}
