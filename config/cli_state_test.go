package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeMachine(t *testing.T, root, key string) {
	t.Helper()
	dir := filepath.Join(root, key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir), []byte("machine_key: "+key+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSelectMachineKey(t *testing.T) {
	root := t.TempDir()
	if _, err := SelectMachineKey(root, ""); !errors.Is(err, ErrNoMachine) {
		t.Fatalf("empty root err = %v", err)
	}
	if _, err := SelectMachineKey(filepath.Join(root, "missing"), ""); !errors.Is(err, ErrNoMachine) {
		t.Fatalf("missing root err = %v", err)
	}

	writeMachine(t, root, "mk_a")
	if key, err := SelectMachineKey(root, ""); err != nil || key != "mk_a" {
		t.Fatalf("sole = %q %v", key, err)
	}

	writeMachine(t, root, "mk_b")
	if _, err := SelectMachineKey(root, ""); !errors.Is(err, ErrMachineAmbiguous) {
		t.Fatalf("ambiguous err = %v", err)
	}
	if err := WriteKeyFile(DefaultPointerPath(root), "mk_b"); err != nil {
		t.Fatal(err)
	}
	if key, err := SelectMachineKey(root, ""); err != nil || key != "mk_b" {
		t.Fatalf("default = %q %v", key, err)
	}
	if key, err := SelectMachineKey(root, "mk_a"); err != nil || key != "mk_a" {
		t.Fatalf("explicit = %q %v", key, err)
	}
	if _, err := SelectMachineKey(root, "mk_x"); !errors.Is(err, ErrNoMachine) {
		t.Fatalf("explicit missing err = %v", err)
	}

	// default 指向已删除的机器时回落到歧义判断。
	if err := WriteKeyFile(DefaultPointerPath(root), "mk_gone"); err != nil {
		t.Fatal(err)
	}
	if _, err := SelectMachineKey(root, ""); !errors.Is(err, ErrMachineAmbiguous) {
		t.Fatalf("stale default err = %v", err)
	}
}

func TestKeyFileRoundTripHasNoNewline(t *testing.T) {
	path := LastMachinePath(t.TempDir())
	if got, err := ReadKeyFile(path); err != nil || got != "" {
		t.Fatalf("missing = %q %v", got, err)
	}
	if err := WriteKeyFile(path, "mk_a"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "mk_a" {
		t.Fatalf("raw = %q", raw)
	}
}

func TestPendingLoginRoundTripAndPerm(t *testing.T) {
	root := t.TempDir()
	if pending, err := LoadPendingLogin(root); err != nil || pending != nil {
		t.Fatalf("missing = %v %v", pending, err)
	}
	want := PendingLogin{DeviceCode: "dc", UserCode: "ABCD-EFGH", PrivateKeyPEM: "priv", PublicKeyPEM: "pub", ExpiresAt: time.Unix(100, 0).UTC(), Interval: 3}
	if err := SavePendingLogin(root, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPendingLogin(root)
	if err != nil || got == nil || *got != want {
		t.Fatalf("got = %+v %v", got, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(PendingLoginPath(root))
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("perm = %v", info.Mode().Perm())
		}
	}
}

func TestEndpointsSnapshotEmptyListIsArray(t *testing.T) {
	dir := t.TempDir()
	if err := SaveEndpoints(dir, EndpointsSnapshot{UpdatedAt: time.Unix(1, 0).UTC()}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(EndpointsPath(dir))
	if !strings.Contains(string(raw), `"endpoints": []`) {
		t.Fatalf("raw = %s", raw)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}

	if err := SaveEndpoints(dir, EndpointsSnapshot{Endpoints: []EndpointEntry{{EndpointID: "ep", Proxies: []EndpointProxy{{ProxyID: "prx_1"}}}}}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadEndpoints(dir)
	if err != nil || !snapshot.HasProxy("prx_1") || snapshot.HasProxy("prx_2") {
		t.Fatalf("snapshot = %+v %v", snapshot, err)
	}
	var missing *EndpointsSnapshot
	if missing.HasProxy("prx_1") {
		t.Fatal("nil snapshot has proxy")
	}
}
