package config

import (
	"path/filepath"
	"testing"
)

func TestResolveServiceRootExplicit(t *testing.T) {
	dir := t.TempDir()
	got, err := ResolveServiceRoot(dir, &ServiceOwner{Home: "/tmp/someone"})
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestResolveServiceRootUsesOwnerHome(t *testing.T) {
	home := t.TempDir()
	got, err := ResolveServiceRoot("", &ServiceOwner{Username: "alice", Home: home})
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(filepath.Join(home, ".orbitproxy"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
