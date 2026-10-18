package runlock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	ModeForeground = "foreground"
	ModeService    = "service"
	// ModeBackground 是 http 等命令拉起的后台 run 进程。
	ModeBackground = "background"
	lockFileName   = "orbitproxy.lock"
)

// Holder is the process that currently owns the machine lock.
type Holder struct {
	PID  int    `json:"pid"`
	Mode string `json:"mode"`
}

// Lock holds an exclusive flock on the machine lock file.
type Lock struct {
	file *os.File
}

// Path is <machineDir>/orbitproxy.lock.
func Path(machineDir string) string {
	return filepath.Join(machineDir, lockFileName)
}

// Acquire takes an exclusive non-blocking lock and records pid/mode.
func Acquire(machineDir, mode string) (*Lock, error) {
	if err := os.MkdirAll(machineDir, 0o700); err != nil {
		return nil, fmt.Errorf("create machine dir: %w", err)
	}
	path := Path(machineDir)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}

	if err := lockExclusive(f); err != nil {
		holder := readHolder(f)
		_ = f.Close()
		if holder != nil {
			return nil, &ConflictError{Holder: *holder}
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}

	holder := Holder{PID: os.Getpid(), Mode: mode}
	if err := writeHolder(f, holder); err != nil {
		_ = unlockFile(f)
		_ = f.Close()
		return nil, err
	}
	return &Lock{file: f}, nil
}

// Close releases the lock.
func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	_ = unlockFile(l.file)
	err := l.file.Close()
	l.file = nil
	return err
}

// Inspect reports the live holder, or nil if the lock is free.
func Inspect(machineDir string) (*Holder, error) {
	path := Path(machineDir)
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if err := lockExclusive(f); err != nil {
		return readHolder(f), nil
	}
	_ = unlockFile(f)
	return nil, nil
}

func writeHolder(f *os.File, holder Holder) error {
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("truncate lock file: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("seek lock file: %w", err)
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(holder); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}
	return f.Sync()
}

func readHolder(f *os.File) *Holder {
	if _, err := f.Seek(0, 0); err != nil {
		return nil
	}
	var holder Holder
	if err := json.NewDecoder(f).Decode(&holder); err != nil {
		return nil
	}
	if holder.PID <= 0 {
		return nil
	}
	return &holder
}

// ConflictError is returned when another orbitproxy process holds the lock.
type ConflictError struct {
	Holder Holder
}

func (e *ConflictError) Error() string {
	return FormatConflict(e.Holder)
}

// FormatConflict explains how to stop the other instance.
func FormatConflict(h Holder) string {
	if h.Mode == ModeService {
		return fmt.Sprintf(
			"OrbitProxy is already running (pid %d, service).\nStop the service first:\n\n  %s\n",
			h.PID,
			serviceStopHint(),
		)
	}
	if h.Mode == ModeBackground {
		return fmt.Sprintf(
			"OrbitProxy is already running in the background (pid %d).\nStop it first:\n\n  orbitproxy stop\n",
			h.PID,
		)
	}
	return fmt.Sprintf(
		"OrbitProxy is already running in the foreground (pid %d).\nPress Ctrl+C in that terminal, then retry.\n",
		h.PID,
	)
}

func serviceStopHint() string {
	if runtime.GOOS == "windows" {
		return "orbitproxy service stop"
	}
	return "sudo orbitproxy service stop"
}
