package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/config"
)

func TestPrintLocalStatusListsEachMachine(t *testing.T) {
	root := t.TempDir()
	writeMinimalConfig(t, root, "mk_a", "edge-a:1")
	writeMinimalConfig(t, root, "mk_b", "edge-b:2")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := printLocalStatus(&buf, root); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "version   -") {
		t.Fatalf("missing version:\n%s", got)
	}
	if !strings.Contains(got, "machines  2") {
		t.Fatalf("missing count:\n%s", got)
	}
	if !strings.Contains(got, "machine   mk_a") || !strings.Contains(got, "machine   mk_b") {
		t.Fatalf("missing machines:\n%s", got)
	}
	if !strings.Contains(got, "edge      edge-a:1") || !strings.Contains(got, "edge      edge-b:2") {
		t.Fatalf("missing edges:\n%s", got)
	}
	if !strings.Contains(got, "process   not running") {
		t.Fatalf("missing process:\n%s", got)
	}
	if strings.Contains(got, "private_key") || strings.Contains(got, "BEGIN") {
		t.Fatalf("status must not print secrets:\n%s", got)
	}
}

func TestPrintLocalStatusShowsStartupFailure(t *testing.T) {
	root := t.TempDir()
	writeMinimalConfig(t, root, "mk_a", "edge-a:1")
	dir := filepath.Join(root, "mk_a")
	logPath := config.LogPath(dir)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		t.Fatal(err)
	}
	line := `time=2026-09-22T22:30:44+08:00 level=ERROR msg="service startup failed" err="open mcp-runtime.env: permission denied"` + "\n"
	if err := os.WriteFile(logPath, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := printLocalStatus(&buf, root); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "process   not running (open mcp-runtime.env: permission denied)") {
		t.Fatalf("missing startup failure:\n%s", buf.String())
	}
}

func TestPrintLocalStatusShowsClientVersion(t *testing.T) {
	buildmeta.SetSoftVersionForTest(t, "1.2.3")
	root := t.TempDir()
	writeMinimalConfig(t, root, "mk_a", "edge-a:1")

	var buf bytes.Buffer
	if err := printLocalStatus(&buf, root); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "version   1.2.3") {
		t.Fatalf("missing client version:\n%s", buf.String())
	}
}

func TestPrintLocalStatusEmpty(t *testing.T) {
	if err := printLocalStatus(os.Stdout, t.TempDir()); err == nil {
		t.Fatal("expected error when no machine config exists")
	}
}

func writeMinimalConfig(t *testing.T, root, key, edge string) {
	t.Helper()
	dir := filepath.Join(root, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	body := "machine_key: " + key + "\nedge_addr: " + edge + "\npublic_key_pem: test\nprivate_key_pem: test\nmachine_ca_cert: test\n"
	if err := os.WriteFile(config.Path(dir), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
