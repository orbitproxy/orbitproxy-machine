package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/config"
)

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
