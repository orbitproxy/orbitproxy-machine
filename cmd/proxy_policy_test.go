package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGeneratePasswordWritesFileWithoutPrintingSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.json")
	body := []byte(`{"rules":[{"type":"basic_auth","name":"review","enabled":true,"users":[{"username":"reviewer"}]}]}`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generateSecret(path, "review", "basic_auth", "users", "username", "reviewer", "password"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Rules []struct {
			Users []struct {
				Password string `json:"password"`
			} `json:"users"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	password := file.Rules[0].Users[0].Password
	if password == "" || strings.Contains("wrote "+path, password) {
		t.Fatal("password missing or leaked into the status line")
	}
}

func TestPolicyApplyJSONIncludesResultAndReviewStatus(t *testing.T) {
	server := policyAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/machine-api/tunnels" {
			writeEnvelope(w, 200, "", map[string]any{"tunnels": []map[string]string{{"proxy_id": "prx_1", "protocol": "https"}}})
			return
		}
		writeEnvelope(w, 200, "", map[string]any{
			"result": "created",
			"rule":   map[string]any{"rule_group_id": "tprg_page", "type": "custom_response", "name": "page", "reviewStatus": "pending"},
		})
	})
	env, out, _ := policyCommandEnv(t, server.URL)
	path := filepath.Join(t.TempDir(), "policy.json")
	body := []byte(`{"rules":[{"type":"custom_response","name":"page","enabled":true}]}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	policyProxyID, policyFile = "prx_1", path
	t.Cleanup(func() { policyProxyID, policyFile = "", "" })
	printer := env.printer(true)
	if err := runCLI(printer, func() error { return runPolicyApply(policyCmd(t), env, printer) }); err != nil {
		t.Fatal(err)
	}
	var got struct {
		OK   bool `json:"ok"`
		Data struct {
			Applied []struct {
				Result       string `json:"result"`
				ReviewStatus string `json:"reviewStatus"`
			} `json:"applied"`
			Failed any `json:"failed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.OK || len(got.Data.Applied) != 1 || got.Data.Applied[0].Result != "created" || got.Data.Applied[0].ReviewStatus != "pending" || got.Data.Failed != nil {
		t.Fatalf("output = %s", out.Bytes())
	}
}

func TestPolicyApplyStopsAndKeepsEarlierRules(t *testing.T) {
	calls := 0
	server := policyAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/machine-api/tunnels" {
			writeEnvelope(w, 200, "", map[string]any{"tunnels": []map[string]string{{"proxy_id": "prx_1", "protocol": "https"}}})
			return
		}
		calls++
		if calls == 1 {
			writeEnvelope(w, 200, "", map[string]any{
				"result": "created",
				"rule":   map[string]any{"rule_group_id": "tprg_1", "type": "ip_access_control", "name": "office"},
			})
			return
		}
		writeEnvelope(w, 400, "invalid_policy_file", nil)
	})
	env, out, _ := policyCommandEnv(t, server.URL)
	path := filepath.Join(t.TempDir(), "policy.json")
	body := []byte(`{"rules":[{"type":"ip_access_control","name":"office","enabled":true},{"type":"basic_auth","name":"review","enabled":true}]}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	policyProxyID, policyFile = "prx_1", path
	t.Cleanup(func() { policyProxyID, policyFile = "", "" })
	printer := env.printer(true)
	err := runCLI(printer, func() error { return runPolicyApply(policyCmd(t), env, printer) })
	code, ok := exitCodeOf(err)
	if !ok || code == 0 {
		t.Fatalf("exit = %v %v", err, code)
	}
	if !strings.Contains(out.String(), `"rule_group_id":"tprg_1"`) || !strings.Contains(out.String(), `"code":"invalid_policy_file"`) || !strings.Contains(out.String(), `"ok":false`) {
		t.Fatalf("output = %s", out.Bytes())
	}
}

func TestPolicyListDeleteMaintenanceAndLogsUseEnvelope(t *testing.T) {
	server := policyAPIServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/machine-api/tunnels/prx_1/policies":
			writeEnvelope(w, 200, "", map[string]any{"rules": []map[string]any{{"rule_group_id": "tprg_1", "type": "ip_access_control", "name": "office", "enabled": true}}})
		case "/v1/machine-api/tunnels/prx_1/policies/ip_access_control/tprg_1":
			writeEnvelope(w, 200, "", map[string]any{"rule_group_id": "tprg_1", "deleted": true})
		case "/v1/machine-api/tunnels/prx_1/maintenance":
			writeEnvelope(w, 200, "", map[string]any{"proxy_id": "prx_1", "public_url": "https://h.example", "maintenance": true})
		case "/v1/machine-api/tunnels/prx_1/access-logs":
			writeEnvelope(w, 200, "", map[string]any{"items": []any{}})
		default:
			writeEnvelope(w, 404, "not_found", nil)
		}
	})
	env, out, _ := policyCommandEnv(t, server.URL)
	policyProxyID = "prx_1"
	policyTypeFlag = "ip_access_control"
	policyRuleID = "tprg_1"
	maintenanceEnable, maintenanceDisable = true, false
	t.Cleanup(func() {
		policyProxyID, policyTypeFlag, policyRuleID = "", "", ""
		maintenanceEnable, maintenanceDisable = false, false
	})
	cmd := policyCmd(t)

	printer := env.printer(true)
	out.Reset()
	if err := runCLI(printer, func() error { return runPolicyList(cmd, env, printer) }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"ok":true`) || !strings.Contains(out.String(), `"tprg_1"`) {
		t.Fatalf("list = %s", out.Bytes())
	}

	printer = env.printer(false)
	out.Reset()
	if err := runCLI(printer, func() error { return runPolicyDelete(cmd, env, printer) }); err != nil {
		t.Fatal(err)
	}
	if out.String() != "deleted tprg_1\n" {
		t.Fatalf("delete = %q", out.String())
	}

	out.Reset()
	if err := runCLI(printer, func() error { return runMaintenance(cmd, env, printer) }); err != nil {
		t.Fatal(err)
	}
	if out.String() != "maintenance enabled\nhttps://h.example\n" {
		t.Fatalf("maintenance = %q", out.String())
	}

	out.Reset()
	if err := runCLI(printer, func() error { return runProxyLogs(cmd, env, printer) }); err != nil {
		t.Fatal(err)
	}
	if out.String() != "no access logs\n" {
		t.Fatalf("logs = %q", out.String())
	}
}

func policyAPIServer(t *testing.T, handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(handle))
	t.Cleanup(server.Close)
	return server
}

func policyCommandEnv(t *testing.T, baseURL string) (*cliEnv, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	env, out, errOut := testEnv(t, baseURL)
	loggedInMachine(t, env, "mk_policy")
	return env, out, errOut
}

func policyCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}
