package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndApplyMcpRuntime(t *testing.T) {
	dir := t.TempDir()
	if err := WriteMcpRuntime(dir, McpRuntime{
		Home:     "/root",
		Path:     "/usr/local/bin:/bin",
		NpmRootG: "/usr/local/lib/node_modules",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, mcpRuntimeFileName)); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", "")
	_ = os.Unsetenv("HOME")
	t.Setenv("PATH", "/bin")
	t.Setenv("ORBITPROXY_NPM_ROOT_G", "")
	_ = os.Unsetenv("ORBITPROXY_NPM_ROOT_G")

	if err := ApplyMcpRuntimeEnv(dir); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("HOME") != "/root" {
		t.Fatalf("HOME=%q", os.Getenv("HOME"))
	}
	if os.Getenv("PATH") != "/bin" {
		t.Fatalf("PATH should stay when already set, got %q", os.Getenv("PATH"))
	}
	if os.Getenv("ORBITPROXY_NPM_ROOT_G") != "/usr/local/lib/node_modules" {
		t.Fatalf("NPM root env=%q", os.Getenv("ORBITPROXY_NPM_ROOT_G"))
	}
}
