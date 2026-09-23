package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logFileName    = "orbitproxy.log"
	fileMaxAgeDays = 7
	fileMaxSizeMB  = 50
)

// Options configures the process slog logger.
type Options struct {
	Level  string
	LogDir string
	// Stdout writes the same greppable text log to stdout (legacy / non-TTY).
	Stdout bool
}

// New builds a slog.Logger that always writes greppable text to LogDir/orbitproxy.log.
func New(opts Options) *slog.Logger {
	level := parseSlogLevel(opts.Level)
	var writers []io.Writer
	if opts.Stdout {
		writers = append(writers, os.Stdout)
	}
	if dir := strings.TrimSpace(opts.LogDir); dir != "" {
		_ = os.MkdirAll(dir, 0o700)
		writers = append(writers, &lumberjack.Logger{
			Filename: filepath.Join(dir, logFileName),
			MaxSize:  fileMaxSizeMB,
			MaxAge:   fileMaxAgeDays,
			Compress: false,
		})
	}
	if len(writers) == 0 {
		writers = []io.Writer{io.Discard}
	}
	return slog.New(slog.NewTextHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level}))
}

func parseSlogLevel(levelStr string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(levelStr)) {
	case "trace", "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error", "err":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Discard returns a logger that drops all records (tests).
func Discard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError + 4,
	}))
}
