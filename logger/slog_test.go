package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWritesGreppableFileLogWithoutANSI(t *testing.T) {
	dir := t.TempDir()
	log := New(Options{Level: "info", LogDir: dir, Stdout: false})
	log.Info("connected", "machine_key", "mk_test")

	body, err := os.ReadFile(filepath.Join(dir, logFileName))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "msg=connected") {
		t.Fatalf("log missing msg: %q", text)
	}
	if !strings.Contains(text, "machine_key=mk_test") {
		t.Fatalf("log missing field: %q", text)
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("file log must not contain ANSI: %q", text)
	}
}

func TestParseSlogLevel(t *testing.T) {
	if parseSlogLevel("DEBUG") != slog.LevelDebug {
		t.Fatalf("debug: got %v", parseSlogLevel("DEBUG"))
	}
	if parseSlogLevel("") != slog.LevelInfo {
		t.Fatalf("default: got %v", parseSlogLevel(""))
	}
}
