// Package buildmeta holds link-time metadata injected via -ldflags.
package buildmeta

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// clientSemverRE 匹配可选 v 前缀的 SemVer。未注入或 dev/devel 一律非法。
var clientSemverRE = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// IsClientSemver 判断 link-time SoftVersion 是否可用来注册。
func IsClientSemver(version string) bool {
	return clientSemverRE.MatchString(version)
}

// ControlPlaneURL is the control-plane base URL, injected at link time.
var ControlPlaneURL = ""

// SoftVersion is the machine client release version, injected at link time.
var SoftVersion = ""

// TLSVerify is injected at link time: "on" enables TLS with certificate verification;
// "off" uses plain TCP (for dev builds connecting directly to edge machineListener).
var TLSVerify = "on"

// TLSVerifyEnabled reports whether the build requires TLS to edge with cert verification.
func TLSVerifyEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(TLSVerify), "off")
}

// Validate normalizes and checks link-time metadata before the process starts.
func Validate() error {
	controlPlaneURL := strings.TrimRight(strings.TrimSpace(ControlPlaneURL), "/")
	if controlPlaneURL == "" {
		return fmt.Errorf("control-plane URL is required; inject buildmeta.ControlPlaneURL via -ldflags")
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
	if !IsClientSemver(SoftVersion) {
		return fmt.Errorf("client version is required; inject buildmeta.SoftVersion via -ldflags as semver, got %q", SoftVersion)
	}

	tlsVerify := strings.ToLower(strings.TrimSpace(TLSVerify))
	if tlsVerify == "" {
		tlsVerify = "on"
	}
	if tlsVerify != "on" && tlsVerify != "off" {
		return fmt.Errorf("buildmeta.TLSVerify must be on or off, got %q", TLSVerify)
	}
	TLSVerify = tlsVerify
	return nil
}

// SetControlPlaneURLForTest overrides ControlPlaneURL for the duration of t.
func SetControlPlaneURLForTest(t *testing.T, controlPlaneURL string) {
	t.Helper()
	prev := ControlPlaneURL
	ControlPlaneURL = strings.TrimRight(strings.TrimSpace(controlPlaneURL), "/")
	t.Cleanup(func() { ControlPlaneURL = prev })
}

// SetTLSVerifyForTest overrides TLSVerify for the duration of t.
func SetTLSVerifyForTest(t *testing.T, tlsVerify string) {
	t.Helper()
	prev := TLSVerify
	TLSVerify = tlsVerify
	t.Cleanup(func() { TLSVerify = prev })
}

// SetSoftVersionForTest overrides SoftVersion for the duration of t.
func SetSoftVersionForTest(t *testing.T, version string) {
	t.Helper()
	prev := SoftVersion
	SoftVersion = version
	t.Cleanup(func() { SoftVersion = prev })
}
