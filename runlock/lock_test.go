package runlock

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquireConflict(t *testing.T) {
	dir := t.TempDir()
	first, err := Acquire(dir, ModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()

	_, err = Acquire(dir, ModeService)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Acquire error = %v, want ConflictError", err)
	}
	if conflict.Holder.PID != os.Getpid() {
		t.Fatalf("pid = %d, want %d", conflict.Holder.PID, os.Getpid())
	}
	if conflict.Holder.Mode != ModeForeground {
		t.Fatalf("mode = %s, want %s", conflict.Holder.Mode, ModeForeground)
	}
	if !strings.Contains(conflict.Error(), "foreground") {
		t.Fatalf("error = %q, want foreground hint", conflict.Error())
	}
}

func TestInspectFreeAndHeld(t *testing.T) {
	dir := t.TempDir()
	holder, err := Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if holder != nil {
		t.Fatalf("Inspect empty dir = %+v, want nil", holder)
	}

	lock, err := Acquire(dir, ModeService)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()

	holder, err = Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if holder == nil || holder.Mode != ModeService {
		t.Fatalf("Inspect held = %+v, want service", holder)
	}
}

func TestAcquireAfterRelease(t *testing.T) {
	dir := t.TempDir()
	first, err := Acquire(dir, ModeForeground)
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := Acquire(dir, ModeService)
	if err != nil {
		t.Fatal(err)
	}
	_ = second.Close()

	if _, err := os.Stat(filepath.Join(dir, lockFileName)); err != nil {
		t.Fatal(err)
	}
}
