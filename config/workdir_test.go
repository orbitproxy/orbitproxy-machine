package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMachineDirSole(t *testing.T) {
	root := t.TempDir()
	machineDir := filepath.Join(root, "mk_a")
	if err := os.MkdirAll(machineDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(machineDir), []byte("machine_key: mk_a\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveMachineDir(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != machineDir {
		t.Fatalf("got %s want %s", got, machineDir)
	}
}

func TestResolveMachineDirMultipleRequiresKey(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"mk_a", "mk_b"} {
		dir := filepath.Join(root, key)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(Path(dir), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ResolveMachineDir(root, ""); err == nil {
		t.Fatal("expected error for multiple machines")
	}
	got, err := ResolveMachineDir(root, "mk_b")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "mk_b" {
		t.Fatalf("got %s", got)
	}
}

func TestListMachineKeysSorted(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"mk_b", "mk_a"} {
		dir := filepath.Join(root, key)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(Path(dir), []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := ListMachineKeys(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "mk_a" || got[1] != "mk_b" {
		t.Fatalf("got %v", got)
	}
}

func TestLogPathUnderLogs(t *testing.T) {
	machineDir := filepath.Join(t.TempDir(), "mk_x")
	got := LogPath(machineDir)
	want := filepath.Join(machineDir, "logs", "orbitproxy.log")
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
