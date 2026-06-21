package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logFileName     = "orbitlink.log"
	defaultLogLevel = "info"
	fileMaxAgeDays  = 7
	fileMaxSizeMB   = 50
)

// Target 指定日志输出目标。
type Target int

const (
	Terminal Target = iota + 1
	File
	Multi
)

type OrbitLinkLogger struct {
	z zerolog.Logger
}

var (
	terminalLogger = OrbitLinkLogger{z: zerolog.Nop()}
	fileLogger     = OrbitLinkLogger{z: zerolog.Nop()}
	multiLogger    = OrbitLinkLogger{z: zerolog.Nop()}

	mu          sync.Mutex
	fileRotator *lumberjack.Logger
	stdoutW     = zerolog.ConsoleWriter{
		Out:         os.Stdout,
		TimeFormat:  "01-02T15:04:05",
		FormatLevel: formatConsoleLevel,
	}
)

func formatConsoleLevel(i interface{}) string {
	levelStr, ok := i.(string)
	if !ok {
		return fmt.Sprintf("%v", i)
	}

	level, err := zerolog.ParseLevel(levelStr)
	if err != nil {
		return levelStr
	}

	label, ok := zerolog.FormattedLevels[level]
	if !ok {
		return levelStr
	}
	if level == zerolog.ErrorLevel {
		label = "err"
	}

	color, ok := zerolog.LevelColors[level]
	if !ok || color == 0 {
		return label
	}
	return colorizeConsole(label, color)
}

func colorizeConsole(s string, c int) string {
	if os.Getenv("NO_COLOR") != "" {
		return s
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", c, s)
}

// Init 初始化 logger；失败时直接 log.Fatal 退出进程。
func Init(level string, workDir string) {
	level = strings.TrimSpace(level)
	if level == "" {
		level = defaultLogLevel
	}

	parsed, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		log.Fatalf("init logger: parse log level: %v", err)
	}

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		log.Fatalf("init logger: create workdir: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if fileRotator != nil {
		_ = fileRotator.Close()
	}

	fileRotator = &lumberjack.Logger{
		Filename:   filepath.Join(workDir, logFileName),
		MaxSize:    fileMaxSizeMB,
		MaxAge:     fileMaxAgeDays,
		MaxBackups: 0,
		LocalTime:  true,
		Compress:   false,
	}

	terminal := zerolog.New(stdoutW).Level(parsed).With().Timestamp().Logger()
	fileOnly := zerolog.New(fileRotator).Level(parsed).With().Timestamp().Logger()
	multi := zerolog.New(zerolog.MultiLevelWriter(stdoutW, fileRotator)).
		Level(parsed).
		With().
		Timestamp().
		Logger()

	terminalLogger = OrbitLinkLogger{z: terminal}
	fileLogger = OrbitLinkLogger{z: fileOnly}
	multiLogger = OrbitLinkLogger{z: multi}
}

func Info(target Target) *zerolog.Event  { return pick(target).Info() }
func Debug(target Target) *zerolog.Event { return pick(target).Debug() }
func Warn(target Target) *zerolog.Event  { return pick(target).Warn() }
func Error(target Target) *zerolog.Event { return pick(target).Error() }
func Fatal(target Target) *zerolog.Event { return pick(target).Fatal() }

// PrintDetailLines 在终端逐行输出 key: value。
func PrintDetailLines(pairs [][2]string) {
	for _, pair := range pairs {
		if pair[1] == "" {
			continue
		}
		fmt.Printf("%s: %s\n", pair[0], pair[1])
	}
}

func pick(target Target) *zerolog.Logger {
	mu.Lock()
	defer mu.Unlock()

	switch target {
	case Terminal:
		return &terminalLogger.z
	case File:
		return &fileLogger.z
	default:
		return &multiLogger.z
	}
}

// Close 关闭日志文件句柄。
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if fileRotator != nil {
		_ = fileRotator.Close()
		fileRotator = nil
	}
	terminalLogger = OrbitLinkLogger{z: zerolog.Nop()}
	fileLogger = OrbitLinkLogger{z: zerolog.Nop()}
	multiLogger = OrbitLinkLogger{z: zerolog.Nop()}
}

// MultiLogger 返回 Multi 目标的 zerolog.Logger，供 app/edgeconn 使用。
func MultiLogger() zerolog.Logger {
	mu.Lock()
	defer mu.Unlock()
	return multiLogger.z
}
