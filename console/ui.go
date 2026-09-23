package console

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	maxEvents = 8

	altEnter  = "\x1b[?1049h"
	altLeave  = "\x1b[?1049l"
	homeErase = "\x1b[H\x1b[J"
)

// UI writes one live status region and rewrites it in place.
type UI struct {
	mu     sync.Mutex
	out    io.Writer
	color  bool
	now    func() time.Time
	status Status
	events []string
	closed bool
}

// NewUI takes over the alternate screen so every update replaces the same card.
func NewUI(out io.Writer, color bool) *UI {
	_, _ = fmt.Fprint(out, altEnter+homeErase)
	return &UI{out: out, color: color, now: time.Now}
}

// Close restores the previous terminal screen.
func (u *UI) Close() {
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed {
		return
	}
	u.closed = true
	_, _ = fmt.Fprint(u.out, altLeave)
}

// Card updates the status snapshot and redraws the live region.
func (u *UI) Card(s Status) {
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status = s
	u.redrawLocked()
}

// Event updates the status snapshot, appends one line, and redraws.
func (u *UI) Event(s Status, msg string) {
	if u == nil {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status = s
	u.events = append(u.events, FormatEvent(u.now(), msg, u.color))
	if len(u.events) > maxEvents {
		u.events = u.events[len(u.events)-maxEvents:]
	}
	u.redrawLocked()
}

func (u *UI) redrawLocked() {
	if u.closed {
		return
	}
	var b strings.Builder
	b.WriteString(FormatCard(u.status, u.color))
	for _, ev := range u.events {
		b.WriteString(ev)
	}
	_, _ = fmt.Fprint(u.out, homeErase)
	if u.color {
		_, _ = fmt.Fprint(u.out, reset)
	}
	_, _ = fmt.Fprint(u.out, b.String())
}
