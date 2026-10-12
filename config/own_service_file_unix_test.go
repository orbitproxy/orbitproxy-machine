//go:build darwin || linux

package config

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestOwnServiceFileToCurrentUser(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, mcpRuntimeFileName)
	if err := os.WriteFile(path, []byte("HOME=/tmp\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := OwnServiceFile(path, &ServiceOwner{Username: current.Username, Home: current.HomeDir}); err != nil {
		t.Fatal(err)
	}
}

func TestOwnServiceTreeLeavesUserFiles(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, "orbitproxy.log")
	if err := os.WriteFile(logPath, []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ownedByRoot, err := fileOwnedByRoot(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if ownedByRoot {
		t.Fatal("temp log should belong to the current user")
	}
	if err := OwnServiceTree(dir, &ServiceOwner{Username: current.Username, Home: current.HomeDir}); err != nil {
		t.Fatal(err)
	}
	if err := OwnServiceTree(dir, &ServiceOwner{Username: "root"}); err != nil {
		t.Fatal(err)
	}
}
