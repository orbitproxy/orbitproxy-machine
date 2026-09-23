//go:build linux

package main

import (
	"strings"
	"testing"
)

func TestLinuxUnitPinsUserAndWorkdir(t *testing.T) {
	unit := linuxUnit(serviceInstallArgs{
		Exe:        "/home/alice/.local/bin/orbitproxy",
		Workdir:    "/home/alice/.orbitproxy",
		MachineKey: "ck_abc",
		Username:   "alice",
		Group:      "alice",
	})
	for _, want := range []string{
		"User=alice",
		"Group=alice",
		"--workdir",
		"/home/alice/.orbitproxy",
		"--machine-key",
		"ck_abc",
		"--from-service",
		"Restart=on-failure",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
	if strings.Contains(unit, "User=root") {
		t.Fatal("unit should not set User=root")
	}
}

func TestLinuxUnitOmitsRootUser(t *testing.T) {
	unit := linuxUnit(serviceInstallArgs{
		Exe:        "/usr/bin/orbitproxy",
		Workdir:    "/root/.orbitproxy",
		MachineKey: "ck_abc",
		Username:   "root",
	})
	if strings.Contains(unit, "User=") {
		t.Fatalf("root unit should omit User=: %s", unit)
	}
}

func TestLinuxUnitWritesHomeAndPath(t *testing.T) {
	unit := linuxUnit(serviceInstallArgs{
		Exe:        "/usr/bin/orbitproxy",
		Workdir:    "/root/.orbitproxy",
		MachineKey: "ck_abc",
		Username:   "root",
		Home:       "/root",
		Path:       "/usr/local/bin:/usr/bin",
	})
	for _, want := range []string{
		`Environment="HOME=/root"`,
		`Environment="PATH=/usr/local/bin:/usr/bin"`,
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("unit missing %q:\n%s", want, unit)
		}
	}
	if strings.Contains(unit, "User=") {
		t.Fatalf("root unit should omit User=: %s", unit)
	}
}

func TestLinuxUnitHomeDiffersByUser(t *testing.T) {
	rootUnit := linuxUnit(serviceInstallArgs{Exe: "/bin/orbitproxy", Workdir: "/root/.orbitproxy", MachineKey: "ck", Username: "root", Home: "/root"})
	aliceUnit := linuxUnit(serviceInstallArgs{Exe: "/bin/orbitproxy", Workdir: "/home/alice/.orbitproxy", MachineKey: "ck", Username: "alice", Home: "/home/alice"})
	if !strings.Contains(rootUnit, `Environment="HOME=/root"`) {
		t.Fatalf("root home missing:\n%s", rootUnit)
	}
	if !strings.Contains(aliceUnit, `Environment="HOME=/home/alice"`) {
		t.Fatalf("alice home missing:\n%s", aliceUnit)
	}
}
