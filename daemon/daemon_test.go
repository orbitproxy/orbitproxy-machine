package daemon

import (
	"errors"
	"testing"

	"github.com/orbitproxy/orbitproxy-machine/runlock"
)

func TestEnsureRunningSkipsWhenServiceHoldsLock(t *testing.T) {
	dir := t.TempDir()
	lock, err := runlock.Acquire(dir, runlock.ModeService)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()

	spawned := false
	holder, started, err := EnsureRunning(Options{
		Root:       t.TempDir(),
		MachineKey: "mk_a",
		MachineDir: dir,
		Executable: "/bin/orbitproxy",
		Spawn: func(string, []string, string) (int, error) {
			spawned = true
			return 1, nil
		},
	})
	if err != nil || started || spawned || holder == nil || holder.Mode != runlock.ModeService {
		t.Fatalf("holder=%+v started=%v spawned=%v err=%v", holder, started, spawned, err)
	}
}

func TestEnsureRunningSpawnsBackgroundRun(t *testing.T) {
	dir := t.TempDir()
	var gotExe, gotOutput string
	var gotArgs []string
	holder, started, err := EnsureRunning(Options{
		Root:       "/root-dir",
		MachineKey: "mk_a",
		MachineDir: dir,
		Executable: "/bin/orbitproxy",
		Spawn: func(exe string, args []string, output string) (int, error) {
			gotExe, gotArgs, gotOutput = exe, args, output
			return 42, nil
		},
	})
	if err != nil || !started || holder.PID != 42 || holder.Mode != runlock.ModeBackground {
		t.Fatalf("holder=%+v started=%v err=%v", holder, started, err)
	}
	want := []string{"--workdir", "/root-dir", "run", "--machine-key", "mk_a", "--log=legacy", "--background-child"}
	if gotExe != "/bin/orbitproxy" || len(gotArgs) != len(want) || gotOutput != OutputPath(dir) {
		t.Fatalf("exe=%s args=%v output=%s", gotExe, gotArgs, gotOutput)
	}
	for i := range want {
		if gotArgs[i] != want[i] {
			t.Fatalf("args = %v", gotArgs)
		}
	}
}

func TestStopRefusesNonBackground(t *testing.T) {
	dir := t.TempDir()
	if _, err := Stop(dir, 0); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("free lock err = %v", err)
	}
	lock, err := runlock.Acquire(dir, runlock.ModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	holder, err := Stop(dir, 0)
	if !errors.Is(err, ErrNotBackground) || holder == nil || holder.Mode != runlock.ModeForeground {
		t.Fatalf("holder=%+v err=%v", holder, err)
	}
}
