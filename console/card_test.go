package console

import (
	"strings"
	"testing"
	"time"
)

func TestFormatCardPlain(t *testing.T) {
	got := FormatCard(Status{
		Version:    "1.2.3",
		State:      "online",
		MachineKey: "mk_test",
		EdgeAddr:   "edge.example:7443",
		SessionID:  "sess_1",
		LogsDir:    "/tmp/logs",
		Forwards: []Forward{{
			Kind:       "basic",
			EndpointID: "mep_1",
			Local:      "127.0.0.1:3000",
		}},
	}, false)
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("plain card must not contain ANSI:\n%s", got)
	}
	want := []string{
		"orbitproxy  1.2.3",
		"● online",
		"Machine",
		"mk_test",
		"edge.example:7443",
		"sess_1",
		"/tmp/logs",
		"Forwarding",
		"basic",
		"mep_1",
		"127.0.0.1:3000",
	}
	for _, part := range want {
		if !strings.Contains(got, part) {
			t.Fatalf("card missing %q:\n%s", part, got)
		}
	}
}

func TestFormatCardWaiting(t *testing.T) {
	got := FormatCard(Status{State: "connecting", MachineKey: "mk"}, false)
	if !strings.Contains(got, "(waiting for endpoints)") {
		t.Fatalf("expected waiting row:\n%s", got)
	}
}

func TestFormatCardColorUsesANSI(t *testing.T) {
	got := FormatCard(Status{State: "online", Version: "1.0.0", MachineKey: "mk"}, true)
	if !strings.Contains(got, green) {
		t.Fatalf("colored online should use green:\n%q", got)
	}
	if !strings.Contains(got, red+"orbit"+reset) || !strings.Contains(got, white+"proxy"+reset) {
		t.Fatalf("brand should be red orbit + white proxy:\n%q", got)
	}
	if !strings.Contains(got, magenta) {
		t.Fatalf("row labels should use magenta:\n%q", got)
	}
	if strings.Contains(got, "\x1b[2m") || strings.Contains(got, "\x1b[3m") {
		t.Fatalf("card must not use dim or italic:\n%q", got)
	}
	if !strings.Contains(got, statusDot+" online") {
		t.Fatalf("colored online should include status dot:\n%q", got)
	}
}

func TestFormatEventPlain(t *testing.T) {
	ts := time.Date(2026, 8, 27, 16, 21, 11, 0, time.UTC)
	got := FormatEvent(ts, "reconnecting  attempt 1  eof", false)
	if got != "16:21:11  reconnecting  attempt 1  eof\n" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatCardLabelsFlushLeft(t *testing.T) {
	got := FormatCard(Status{
		Version:    "1.0.6",
		State:      "online",
		MachineKey: "mk",
		EdgeAddr:   "edge:1",
		LogsDir:    "/tmp/logs",
	}, false)
	for _, label := range []string{"Machine", "Edge", "Logs", "Forwarding"} {
		if !strings.Contains(got, "\n"+label) {
			t.Fatalf("%s should start at the left edge:\n%s", label, got)
		}
	}
	waiting := FormatCard(Status{State: "online", Forwards: []Forward{{Kind: "basic", Local: "127.0.0.1:3000"}}}, false)
	if !strings.Contains(waiting, "\nbasic") {
		t.Fatalf("forwarding rows should start at the left edge:\n%s", waiting)
	}
}

func TestParseLogMode(t *testing.T) {
	legacy, err := ParseLogMode("")
	if err != nil || legacy {
		t.Fatalf("empty: legacy=%v err=%v", legacy, err)
	}
	legacy, err = ParseLogMode("legacy")
	if err != nil || !legacy {
		t.Fatalf("legacy: legacy=%v err=%v", legacy, err)
	}
	if _, err = ParseLogMode("json"); err == nil {
		t.Fatal("expected error for json")
	}
}

func TestUseCard(t *testing.T) {
	if !UseCard(false, false, true) {
		t.Fatal("TTY foreground should use card")
	}
	if UseCard(true, false, true) {
		t.Fatal("legacy should not use card")
	}
	if UseCard(false, true, true) {
		t.Fatal("service should not use card")
	}
	if UseCard(false, false, false) {
		t.Fatal("non-TTY should not use card")
	}
}
