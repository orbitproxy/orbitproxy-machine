//go:build darwin

package main

import (
	"strings"
	"testing"
)

func TestDarwinPlistPinsUserAndWorkdir(t *testing.T) {
	plist := darwinPlist(serviceInstallArgs{
		Exe:        "/Users/alice/.local/bin/orbitproxy",
		Workdir:    "/Users/alice/.orbitproxy",
		MachineKey: "ck_abc",
		Username:   "alice",
		Home:       "/Users/alice",
		Path:       "/usr/local/bin:/usr/bin",
	})
	for _, want := range []string{
		"<string>alice</string>",
		"<string>/Users/alice/.orbitproxy</string>",
		"<string>ck_abc</string>",
		"<string>run</string>",
		"<string>--from-service</string>",
		"<key>RunAtLoad</key>",
		"<key>KeepAlive</key>",
		"<key>EnvironmentVariables</key>",
		"<key>HOME</key>",
		"<string>/Users/alice</string>",
		"<key>PATH</key>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q:\n%s", want, plist)
		}
	}
}
