package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/httpclient"
)

func TestRegisterMachinePersistsConfig(t *testing.T) {
	// Not parallel: registerMachine uses package-level httpclient.SetOverride.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/machines/register" {
			t.Fatalf("path = %q, want /v1/machines/register", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if _, ok := body["machineId"]; ok {
			t.Fatal("register request must not send machineId")
		}
		if _, ok := body["machineKey"]; ok {
			t.Fatal("create register must not send machineKey")
		}
		if body["type"] != "cli" {
			t.Fatalf("type = %v, want cli", body["type"])
		}
		if body["version"] != "1.2.3" {
			t.Fatalf("version = %v, want 1.2.3", body["version"])
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(httpclient.Response[map[string]any]{
			Code: 200,
			Data: map[string]any{
				"machine": map[string]any{
					"id":                   "mach_test",
					"machineKey":           "mk_test",
					"type":                 "cli",
					"publicKeyFingerprint": "fingerprint",
				},
				"edge": map[string]any{
					"addr":   "edge-a.orbitproxy.run:7443",
					"caCert": "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
				},
			},
		})
	}))
	defer server.Close()

	buildmeta.SetControlPlaneURLForTest(t, server.URL)
	buildmeta.SetSoftVersionForTest(t, "1.2.3")

	root := t.TempDir()
	result, err := registerMachine(context.Background(), registerOptions{
		authToken:  "token-1",
		workDir:    root,
		httpClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("registerMachine returned error: %v", err)
	}

	if result.machineKey != "mk_test" {
		t.Fatalf("machineKey = %q, want mk_test", result.machineKey)
	}
	machineDir := filepath.Join(root, "mk_test")
	if result.workDir != machineDir {
		t.Fatalf("workDir = %q, want %q", result.workDir, machineDir)
	}

	machineCfg, err := config.LoadFromFile(config.Path(machineDir))
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}
	if machineCfg.MachineKey != "mk_test" {
		t.Fatalf("stored machine_key = %q, want mk_test", machineCfg.MachineKey)
	}
	if machineCfg.MachineCACert == "" {
		t.Fatal("stored machine_ca_cert is empty")
	}
	if machineCfg.PrivateKeyPEM == "" {
		t.Fatal("stored private key is empty")
	}
	if machineCfg.PublicKeyPEM == "" {
		t.Fatal("stored public key is empty")
	}

	raw, err := os.ReadFile(config.Path(machineDir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "machine_id") {
		t.Fatalf("persisted config must not contain machine_id:\n%s", raw)
	}
	if strings.Contains(string(raw), "client_key") {
		t.Fatalf("persisted config must not contain client_key:\n%s", raw)
	}
}

func TestRegisterMachineRestoresByMachineKey(t *testing.T) {
	// Not parallel: registerMachine uses package-level httpclient.SetOverride.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["machineKey"] != "mk_sandbox" {
			t.Fatalf("machineKey = %v, want mk_sandbox", body["machineKey"])
		}
		if body["version"] != "1.2.3" {
			t.Fatalf("version = %v, want 1.2.3", body["version"])
		}
		if _, ok := body["type"]; ok {
			t.Fatal("restore register must not send type")
		}
		if _, ok := body["machineId"]; ok {
			t.Fatal("register request must not send machineId")
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(httpclient.Response[map[string]any]{
			Code: 200,
			Data: map[string]any{
				"machine": map[string]any{
					"id":                   "mach_sandbox",
					"machineKey":           "mk_sandbox",
					"type":                 "sandbox",
					"publicKeyFingerprint": "fingerprint",
				},
				"edge": map[string]any{
					"addr":   "edge-a.orbitproxy.run:7443",
					"caCert": "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n",
				},
			},
		})
	}))
	defer server.Close()

	buildmeta.SetControlPlaneURLForTest(t, server.URL)
	buildmeta.SetSoftVersionForTest(t, "1.2.3")

	workDir := t.TempDir()
	result, err := registerMachine(context.Background(), registerOptions{
		authToken:  "token-1",
		machineKey: "mk_sandbox",
		workDir:    workDir,
		httpClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("registerMachine returned error: %v", err)
	}
	if result.machineKey != "mk_sandbox" {
		t.Fatalf("machineKey = %q, want mk_sandbox", result.machineKey)
	}
}

func TestRegisterMachineRejectsMissingVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("register must not be sent without a semver version")
	}))
	defer server.Close()

	buildmeta.SetControlPlaneURLForTest(t, server.URL)
	buildmeta.SetSoftVersionForTest(t, "dev")

	_, err := registerMachine(context.Background(), registerOptions{
		authToken:  "token-1",
		workDir:    t.TempDir(),
		httpClient: server.Client(),
	})
	if err == nil {
		t.Fatal("registerMachine unexpectedly succeeded")
	}
}
