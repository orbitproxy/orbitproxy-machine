package config

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestLoadNotFound(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	_, err := Load(workDir)
	if err == nil {
		t.Fatal("Load unexpectedly succeeded")
	}

	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("error type = %T, want *LoadError", err)
	}
	if loadErr.Kind != "not_found" {
		t.Fatalf("Kind = %q, want not_found", loadErr.Kind)
	}
}

func TestLoadIncomplete(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	configPath := Path(workDir)
	if err := Save(workDir, Config{
		MachineKey:    "mk_test",
		EdgeAddr:      "edge.example.com:443",
		MachineCACert: "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	_, err := Load(workDir)
	if err == nil {
		t.Fatal("Load unexpectedly succeeded")
	}

	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("error type = %T, want *LoadError", err)
	}
	if loadErr.Kind != "incomplete" {
		t.Fatalf("Kind = %q, want incomplete", loadErr.Kind)
	}
	if len(loadErr.Missing) != 1 || loadErr.Missing[0] != "private_key_pem" {
		t.Fatalf("Missing = %#v, want [private_key_pem]", loadErr.Missing)
	}
	if loadErr.ConfigPath != configPath {
		t.Fatalf("ConfigPath = %q, want %q", loadErr.ConfigPath, configPath)
	}
}

func TestLoadOK(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cfg := Config{
		MachineKey:    "mk_test",
		PrivateKeyPEM: "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n",
		PublicKeyPEM:  "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----\n",
		EdgeAddr:      "edge.example.com:443",
		MachineCACert: "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
	}
	if err := Save(workDir, cfg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load(workDir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.MachineKey != cfg.MachineKey {
		t.Fatalf("MachineKey = %q, want %q", loaded.MachineKey, cfg.MachineKey)
	}
	if loaded.PrivateKeyPEM != cfg.PrivateKeyPEM {
		t.Fatal("PrivateKeyPEM mismatch")
	}
	if loaded.EdgeAddr != "edge.example.com:443" {
		t.Fatalf("EdgeAddr = %q, want edge.example.com:443", loaded.EdgeAddr)
	}

	if filepath.Base(Path(workDir)) != "orbitproxy.yml" {
		t.Fatalf("unexpected config file name")
	}
}

func TestLoadIncompleteMissingEdgeAddr(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	if err := Save(workDir, Config{
		MachineKey:    "mk_test",
		PrivateKeyPEM: "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----\n",
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	_, err := Load(workDir)
	if err == nil {
		t.Fatal("Load unexpectedly succeeded")
	}

	var loadErr *LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("error type = %T, want *LoadError", err)
	}
	if loadErr.Kind != "incomplete" {
		t.Fatalf("Kind = %q, want incomplete", loadErr.Kind)
	}
	if len(loadErr.Missing) != 2 ||
		loadErr.Missing[0] != "edge_addr" ||
		loadErr.Missing[1] != "machine_ca_cert" {
		t.Fatalf("Missing = %#v, want [edge_addr machine_ca_cert]", loadErr.Missing)
	}
}
