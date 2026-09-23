package console

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	reset   = "\x1b[0m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	cyan    = "\x1b[36m"
	magenta = "\x1b[35m"
	white   = "\x1b[37m"
	gray    = "\x1b[90m"

	statusDot  = "●"
	labelWidth = 10
	kindWidth  = 10
	headerSpan = 56
)

// Status is the snapshot drawn as the foreground card.
type Status struct {
	Version    string
	State      string
	MachineKey string
	EdgeAddr   string
	SessionID  string
	LogsDir    string
	Forwards   []Forward
}

// Forward is one local endpoint row.
type Forward struct {
	Kind       string
	EndpointID string
	Local      string
}

// FormatCard renders a ngrok-style status card. color=false is for tests.
func FormatCard(s Status, color bool) string {
	var b strings.Builder
	state := strings.TrimSpace(s.State)
	if state == "" {
		state = "connecting"
	}
	b.WriteString(formatHeader(s.Version, state, color))
	b.WriteByte('\n')
	writeRow(&b, "Machine", s.MachineKey, color)
	writeRow(&b, "Edge", s.EdgeAddr, color)
	if strings.TrimSpace(s.SessionID) != "" {
		writeRow(&b, "Session", s.SessionID, color)
	}
	if strings.TrimSpace(s.LogsDir) != "" {
		writeRow(&b, "Logs", s.LogsDir, color)
	}
	b.WriteByte('\n')
	b.WriteString(paint(magenta, "Forwarding", color))
	b.WriteByte('\n')
	if len(s.Forwards) == 0 {
		b.WriteString(paint(gray, "(waiting for endpoints)", color))
		b.WriteByte('\n')
	} else {
		for _, f := range s.Forwards {
			kind := fmt.Sprintf("%-*s", kindWidth, emptyDash(f.Kind))
			b.WriteString(paint(cyan, kind, color))
			b.WriteString(paint(cyan, emptyDash(f.Local), color))
			b.WriteByte('\n')
			b.WriteString(fmt.Sprintf("%-*s", kindWidth, ""))
			b.WriteString(paint(gray, emptyDash(f.EndpointID), color))
			b.WriteByte('\n')
		}
	}
	b.WriteByte('\n')
	return b.String()
}

// FormatEvent renders one status line under the card.
func FormatEvent(t time.Time, msg string, color bool) string {
	ts := t.Format("15:04:05")
	return paint(gray, ts, color) + "  " + msg + "\n"
}

func formatHeader(version, state string, color bool) string {
	name := "orbitproxy"
	left := name
	if v := strings.TrimSpace(version); v != "" {
		left += "  " + v
	}
	right := statusDot + " " + state
	gap := headerSpan - len(left) - utf8.RuneCountInString(right)
	if gap < 2 {
		gap = 2
	}
	brand := paint(red, "orbit", color) + paint(white, "proxy", color)
	if v := strings.TrimSpace(version); v != "" {
		brand += "  " + v
	}
	return brand + strings.Repeat(" ", gap) + paintState(state, color) + "\n"
}

func writeRow(b *strings.Builder, label, value string, color bool) {
	if strings.TrimSpace(value) == "" {
		return
	}
	lab := fmt.Sprintf("%-*s", labelWidth, label)
	b.WriteString(paint(magenta, lab, color))
	b.WriteString(value)
	b.WriteByte('\n')
}

func paintState(state string, color bool) string {
	label := statusDot + " " + state
	switch strings.ToLower(state) {
	case "online":
		return paint(green, label, color)
	case "reconnecting":
		return paint(yellow, label, color)
	case "offline":
		return paint(red, label, color)
	default:
		return paint(yellow, label, color)
	}
}

func paint(code, s string, color bool) string {
	if !color || s == "" {
		return s
	}
	return code + s + reset
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
