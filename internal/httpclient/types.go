package httpclient

import (
	"runtime"
	"strings"
)

type Response[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
	Data    T      `json:"data"`
}

type Base struct {
	OS      string `json:"os,omitempty"`
	Arch    string `json:"arch,omitempty"`
	Version string `json:"version,omitempty"`
}

func (b Base) Normalized() Base {
	return Base{
		OS:      strings.TrimSpace(b.OS),
		Arch:    strings.TrimSpace(b.Arch),
		Version: strings.TrimSpace(b.Version),
	}
}

func (b Base) Resolve(defaultVersion string) Base {
	out := b.Normalized()
	if out.OS == "" {
		out.OS = runtime.GOOS
	}
	if out.Arch == "" {
		out.Arch = runtime.GOARCH
	}
	if out.Version == "" {
		out.Version = strings.TrimSpace(defaultVersion)
	}
	return out
}
