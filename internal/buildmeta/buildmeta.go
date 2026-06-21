// Package buildmeta holds link-time metadata injected via -ldflags.
package buildmeta

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// ControlPlaneURL is the control-plane base URL, injected at link time.
var ControlPlaneURL = ""

// SoftVersion is the machine client release version, injected at link time.
var SoftVersion = "dev"

// Validate normalizes and checks link-time metadata before the process starts.
func Validate() error {
	controlPlaneURL := strings.TrimRight(strings.TrimSpace(ControlPlaneURL), "/")
	if controlPlaneURL == "" {
		return fmt.Errorf("control-plane URL is required; inject internal/buildmeta.ControlPlaneURL via -ldflags")
	}

	parsed, err := url.Parse(controlPlaneURL)
	if err != nil {
		return fmt.Errorf("parse control-plane URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("control-plane URL must include scheme and host")
	}

	ControlPlaneURL = controlPlaneURL
	SoftVersion = strings.TrimSpace(SoftVersion)
	if SoftVersion == "" {
		SoftVersion = "dev"
	}
	return nil
}

// SetControlPlaneURLForTest overrides ControlPlaneURL for the duration of t.
func SetControlPlaneURLForTest(t *testing.T, controlPlaneURL string) {
	t.Helper()
	prev := ControlPlaneURL
	ControlPlaneURL = strings.TrimRight(strings.TrimSpace(controlPlaneURL), "/")
	t.Cleanup(func() { ControlPlaneURL = prev })
}
