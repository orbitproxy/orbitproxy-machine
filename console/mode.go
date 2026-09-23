package console

import (
	"fmt"
	"strings"
)

// ParseLogMode accepts the --log flag. Empty is auto (status card on a TTY).
func ParseLogMode(s string) (legacy bool, err error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return false, nil
	case "legacy":
		return true, nil
	default:
		return false, fmt.Errorf(`invalid --log value %q (want "legacy")`, s)
	}
}

// UseCard reports whether stdout should show the status card.
func UseCard(legacy, fromService, stdoutIsTTY bool) bool {
	return !legacy && !fromService && stdoutIsTTY
}
