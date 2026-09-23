package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWipeAllowedRejectsUnsafePaths(t *testing.T) {
	home := t.TempDir()
	if err := wipeAllowed("/", true, home); err == nil {
		t.Fatal("expected refuse to delete /")
	}
	if err := wipeAllowed(home, false, home); err == nil {
		t.Fatal("expected refuse to delete home")
	}
	if err := wipeAllowed(filepath.Join(home, "not-orbit"), true, home); err == nil {
		t.Fatal("expected refuse to delete non-.orbitproxy default root")
	}
	if err := wipeAllowed(filepath.Join(home, ".orbitproxy"), true, home); err != nil {
		t.Fatalf("default data root should be allowed: %v", err)
	}
	if err := wipeAllowed(filepath.Join(home, "custom-data"), false, home); err != nil {
		t.Fatalf("explicit workdir should be allowed: %v", err)
	}
}

func TestPerformResetKeepsProgram(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(root, "bin", "orbitproxy")
	if err := os.WriteFile(binPath, []byte("exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMinimalConfig(t, root, "mk_a", "edge-a:1")

	var buf bytes.Buffer
	err := performPurge(&buf, resetDeps{
		serviceInstalled: func() (bool, error) { return false, nil },
		requireAdmin:     func() error { return nil },
		removeService:    func() error { t.Fatal("removeService"); return nil },
	}, []resetTarget{{path: root, mustBeOrbitRoot: false}}, filepath.Dir(root), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("program was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "mk_a")); !os.IsNotExist(err) {
		t.Fatalf("machine config still exists: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Deleted "+filepath.Join(root, "mk_a")) {
		t.Fatalf("missing delete line:\n%s", got)
	}
	if !strings.Contains(got, "The program was kept") {
		t.Fatalf("missing kept-program line:\n%s", got)
	}
	if strings.Contains(got, "Reinstall from the console") {
		t.Fatalf("reset must not ask for a reinstall:\n%s", got)
	}
}

func TestPerformDeleteRemovesProgram(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "orbitproxy"), []byte("exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeMinimalConfig(t, root, "mk_a", "edge-a:1")

	var buf bytes.Buffer
	err := performPurge(&buf, resetDeps{
		serviceInstalled: func() (bool, error) { return false, nil },
		requireAdmin:     func() error { return nil },
		removeService:    func() error { t.Fatal("removeService"); return nil },
	}, []resetTarget{{path: root, mustBeOrbitRoot: false}}, filepath.Dir(root), true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("root still exists: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Deleted "+root) {
		t.Fatalf("missing delete line:\n%s", got)
	}
	if !strings.Contains(got, "Reinstall from the console") {
		t.Fatalf("missing reinstall hint:\n%s", got)
	}
}

func TestPerformResetRemovesServiceThenFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	removed := false
	var buf bytes.Buffer
	err := performPurge(&buf, resetDeps{
		serviceInstalled: func() (bool, error) { return true, nil },
		requireAdmin:     func() error { return nil },
		removeService: func() error {
			removed = true
			return nil
		},
	}, []resetTarget{{path: root, mustBeOrbitRoot: false}}, filepath.Dir(root), false)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("expected service removal")
	}
	if !strings.Contains(buf.String(), "Removed OS service") {
		t.Fatalf("missing service line:\n%s", buf.String())
	}
}

func TestPerformResetRequiresAdminWhenServiceInstalled(t *testing.T) {
	root := t.TempDir()
	err := performPurge(os.Stdout, resetDeps{
		serviceInstalled: func() (bool, error) { return true, nil },
		requireAdmin:     func() error { return errServiceUnsupported },
		removeService:    func() error { t.Fatal("must not remove service"); return nil },
	}, []resetTarget{{path: root, mustBeOrbitRoot: false}}, filepath.Dir(root), false)
	if err == nil {
		t.Fatal("expected admin error")
	}
	if !strings.Contains(err.Error(), "system service") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(root); statErr != nil {
		t.Fatalf("must not delete files when service uninstall is blocked: %v", statErr)
	}
}

func TestPerformResetIdempotentWhenEmpty(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "gone")
	var buf bytes.Buffer
	err := performPurge(&buf, resetDeps{
		serviceInstalled: func() (bool, error) { return false, nil },
		requireAdmin:     func() error { return nil },
		removeService:    func() error { t.Fatal("removeService"); return nil },
	}, []resetTarget{{path: missing, mustBeOrbitRoot: false}}, t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Nothing to remove") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestStopLocalProcessesKillsExecPID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix process signal test")
	}
	root := t.TempDir()
	machineDir := filepath.Join(root, "mk_a")
	if err := os.MkdirAll(machineDir, 0o700); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	body, err := json.Marshal([]map[string]int{{"pid": cmd.Process.Pid}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(machineDir, "exec_pids.json"), body, 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := stopLocalProcesses(&buf, root); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Stopped process pid") {
		t.Fatalf("missing stop line:\n%s", buf.String())
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("process still running")
	}
}

func TestResolveResetTargetsAddsInstallRoot(t *testing.T) {
	custom := t.TempDir()
	targets, home, err := resolveResetTargets(custom, true)
	if err != nil {
		t.Fatal(err)
	}
	if home == "" {
		t.Fatal("expected home")
	}
	if len(targets) < 1 {
		t.Fatal("expected at least the explicit workdir")
	}
	if filepath.Clean(targets[0].path) != filepath.Clean(custom) {
		t.Fatalf("first target = %q, want %q", targets[0].path, custom)
	}
	if targets[0].mustBeOrbitRoot {
		t.Fatal("explicit workdir must not require .orbitproxy basename")
	}
	if len(targets) == 2 && !targets[1].mustBeOrbitRoot {
		t.Fatal("install root must require .orbitproxy basename")
	}
}

func TestResolveResetTargetsSkipsInstallRoot(t *testing.T) {
	custom := t.TempDir()
	targets, _, err := resolveResetTargets(custom, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("reset should not include the install root, got %d targets", len(targets))
	}
}
