package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/orbitlink/orbitlink-machine/internal/buildmeta"
	"github.com/orbitlink/orbitlink-machine/internal/config"
	"github.com/orbitlink/orbitlink-machine/internal/httpclient"
)

func TestRegisterMachinePersistsConfig(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/machines/register" {
			t.Fatalf("path = %q, want /v1/machines/register", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(httpclient.Response[map[string]any]{
			Code: 200,
			Data: map[string]any{
				"machine": map[string]any{
					"id":                   "mach_test",
					"machineId":            "mach_test_local",
					"publicKeyFingerprint": "fingerprint",
				},
				"edge": map[string]any{
					"addr": "edge-a.orbitlink.run:7443",
					"tls": map[string]any{
						"serverName": "edge-a.orbitlink.run",
					},
				},
			},
		})
	}))
	defer server.Close()

	buildmeta.SetControlPlaneURLForTest(t, server.URL)

	workDir := t.TempDir()
	result, err := registerMachine(context.Background(), registerOptions{
		authToken: "token-1",
		workDir:   workDir,
		machineID: "mach_test_local",
		httpClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("registerMachine returned error: %v", err)
	}

	if result.machineID != "mach_test" {
		t.Fatalf("machineID = %q, want mach_test", result.machineID)
	}

	machineCfg, err := config.LoadFromFile(config.Path(workDir))
	if err != nil {
		t.Fatalf("LoadFromFile returned error: %v", err)
	}
	if machineCfg.MachineID != "mach_test" {
		t.Fatalf("stored machine_id = %q, want mach_test", machineCfg.MachineID)
	}
	if machineCfg.PrivateKeyPEM == "" {
		t.Fatal("stored private key is empty")
	}
	if machineCfg.PublicKeyPEM == "" {
		t.Fatal("stored public key is empty")
	}
}
