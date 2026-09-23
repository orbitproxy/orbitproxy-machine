package console

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestUIRedrawsFromHome(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUI(&buf, false)
	defer ui.Close()
	ui.now = func() time.Time {
		return time.Date(2026, 8, 28, 14, 3, 31, 0, time.UTC)
	}
	if !strings.Contains(buf.String(), altEnter) {
		t.Fatalf("NewUI should enter the alternate screen:\n%q", buf.String())
	}

	ui.Card(Status{Version: "1.0.6", State: "connecting", MachineKey: "mk"})
	first := buf.String()
	if strings.Count(first, "orbitproxy") != 1 || !strings.Contains(first, "● connecting") {
		t.Fatalf("first frame missing connecting card:\n%s", first)
	}

	ui.Card(Status{Version: "1.0.6", State: "online", MachineKey: "mk", SessionID: "sess_1"})
	rewrite := buf.String()[len(first):]
	if !strings.HasPrefix(rewrite, homeErase) {
		t.Fatalf("update should home and erase, not append:\n%q", rewrite)
	}
	if strings.Contains(rewrite, "● connecting") {
		t.Fatalf("rewrite should drop connecting:\n%s", rewrite)
	}
	if !strings.Contains(rewrite, "● online") {
		t.Fatalf("rewrite missing online:\n%s", rewrite)
	}
}

func TestUIEventRewritesSameHeader(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUI(&buf, false)
	defer ui.Close()
	ui.now = func() time.Time {
		return time.Date(2026, 8, 28, 14, 3, 31, 0, time.UTC)
	}
	ui.Card(Status{Version: "1.0.6", State: "online", MachineKey: "mk"})
	firstLen := buf.Len()
	ui.Event(Status{Version: "1.0.6", State: "reconnecting", MachineKey: "mk"}, "reconnecting  attempt 1  eof")
	rewrite := buf.String()[firstLen:]
	if strings.Count(rewrite, "orbitproxy") != 1 {
		t.Fatalf("event should rewrite one card:\n%s", rewrite)
	}
	if !strings.Contains(rewrite, "● reconnecting") {
		t.Fatalf("header should switch to reconnecting:\n%s", rewrite)
	}
	if !strings.Contains(rewrite, "14:03:31  reconnecting  attempt 1  eof") {
		t.Fatalf("event line missing:\n%s", rewrite)
	}
}

func TestUICloseLeavesAltScreen(t *testing.T) {
	var buf bytes.Buffer
	ui := NewUI(&buf, false)
	ui.Close()
	if !strings.HasSuffix(buf.String(), altLeave) {
		t.Fatalf("Close should leave the alternate screen:\n%q", buf.String())
	}
	ui.Card(Status{State: "online"})
	if strings.Count(buf.String(), "orbitproxy") != 0 {
		t.Fatal("Card after Close must not paint")
	}
}
