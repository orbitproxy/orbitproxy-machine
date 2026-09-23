package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/config"
)

const (
	serviceConnectWait = 12 * time.Second
	serviceConnectPoll = 200 * time.Millisecond
)

func logFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func reportServiceConnect(out io.Writer, machineDir string, offset int64) error {
	logPath := config.LogPath(machineDir)
	deadline := time.Now().Add(serviceConnectWait)
	for {
		result, err := inspectServiceLogSince(logPath, offset)
		if err != nil {
			return err
		}
		switch {
		case result.success != "":
			fmt.Fprintln(out, result.success)
			return nil
		case result.fail != "":
			return fmt.Errorf("%s\nlog: %s", result.fail, logPath)
		}
		if !time.Now().Before(deadline) {
			fmt.Fprintf(out, "service is running; no connect result yet. See %s\n", logPath)
			return nil
		}
		time.Sleep(serviceConnectPoll)
	}
}

type serviceLogResult struct {
	success string
	fail    string
}

func inspectServiceLogSince(path string, offset int64) (serviceLogResult, error) {
	data, err := readFileFrom(path, offset)
	if err != nil {
		if os.IsNotExist(err) {
			return serviceLogResult{}, nil
		}
		return serviceLogResult{}, err
	}
	return parseServiceConnectLog(string(data)), nil
}

func readFileFrom(path string, offset int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}
	}
	return io.ReadAll(f)
}

func parseServiceConnectLog(chunk string) serviceLogResult {
	var fail string
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		msg, _ := slogAttr(line, "msg")
		switch msg {
		case "login to edge success":
			return serviceLogResult{success: "login to edge success"}
		case "connect to edge error", "connect to edge failed, retrying":
			if errText, ok := slogAttr(line, "err"); ok && errText != "" {
				fail = errText
			} else {
				fail = msg
			}
		case "edge session permanently closed, stop reconnecting":
			if reason, ok := slogAttr(line, "reason"); ok && reason != "" {
				fail = reason
			} else {
				fail = msg
			}
		case "service startup failed":
			if errText, ok := slogAttr(line, "err"); ok && errText != "" {
				fail = errText
			} else {
				fail = msg
			}
		}
	}
	if fail != "" {
		return serviceLogResult{fail: fail}
	}
	return serviceLogResult{}
}

func slogAttr(line, key string) (string, bool) {
	prefix := key + "="
	idx := indexSlogKey(line, prefix)
	if idx < 0 {
		return "", false
	}
	rest := line[idx+len(prefix):]
	if rest == "" {
		return "", true
	}
	if rest[0] != '"' {
		end := strings.IndexByte(rest, ' ')
		if end < 0 {
			return rest, true
		}
		return rest[:end], true
	}
	var b strings.Builder
	escaped := false
	for i := 1; i < len(rest); i++ {
		ch := rest[i]
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return b.String(), true
		}
		b.WriteByte(ch)
	}
	return b.String(), true
}

func noteServiceStartupFailure(machineDir string, cause error) {
	if cause == nil || strings.TrimSpace(machineDir) == "" {
		return
	}
	logPath := config.LogPath(machineDir)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "time=%s level=ERROR msg=%q err=%q\n",
		time.Now().Format(time.RFC3339),
		"service startup failed",
		cause.Error(),
	)
}

func lastUnrecoveredStartupFailure(chunk string) string {
	var fail string
	recovered := false
	for _, line := range strings.Split(chunk, "\n") {
		line = strings.TrimRight(line, "\r")
		msg, ok := slogAttr(line, "msg")
		if !ok {
			continue
		}
		switch msg {
		case "service startup failed":
			if errText, ok := slogAttr(line, "err"); ok && errText != "" {
				fail = errText
			} else {
				fail = msg
			}
			recovered = false
		case "orbitproxy started", "login to edge success":
			recovered = true
		}
	}
	if recovered {
		return ""
	}
	return fail
}

func indexSlogKey(line, prefix string) int {
	if strings.HasPrefix(line, prefix) {
		return 0
	}
	idx := strings.Index(line, " "+prefix)
	if idx < 0 {
		return -1
	}
	return idx + 1
}
