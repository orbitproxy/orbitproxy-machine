//go:build darwin

package main

import (
	"os"
	"strings"
	"testing"
)

func TestRefuseRootForegroundWhenNotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	if err := refuseRootForeground(); err != nil {
		t.Fatal(err)
	}
}

func TestRootForegroundMessage(t *testing.T) {
	err := refuseRootForeground()
	if os.Geteuid() != 0 {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), "must not be started with sudo") {
		t.Fatalf("got %v", err)
	}
}
