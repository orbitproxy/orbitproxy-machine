package runlock

import (
	"strings"
	"testing"
)

func TestFormatConflictForeground(t *testing.T) {
	msg := FormatConflict(Holder{PID: 42, Mode: ModeForeground})
	if !strings.Contains(msg, "foreground") || !strings.Contains(msg, "42") {
		t.Fatalf("msg = %q", msg)
	}
	if !strings.Contains(msg, "Ctrl+C") {
		t.Fatalf("msg = %q, want Ctrl+C hint", msg)
	}
}

func TestFormatConflictBackground(t *testing.T) {
	msg := FormatConflict(Holder{PID: 7, Mode: ModeBackground})
	if !strings.Contains(msg, "background") || !strings.Contains(msg, "orbitproxy stop") {
		t.Fatalf("msg = %q", msg)
	}
}

func TestFormatConflictService(t *testing.T) {
	msg := FormatConflict(Holder{PID: 9, Mode: ModeService})
	if !strings.Contains(msg, "service") || !strings.Contains(msg, "9") {
		t.Fatalf("msg = %q", msg)
	}
	if !strings.Contains(msg, "orbitproxy service stop") {
		t.Fatalf("msg = %q, want service stop hint", msg)
	}
}
