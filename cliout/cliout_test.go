package cliout

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/orbitproxy/orbitproxy-machine/machineapi"
)

func TestExitCodeMapping(t *testing.T) {
	cases := map[string]int{
		"invalid_port":            ExitUsage,
		"invalid_host":            ExitUsage,
		"invalid_subdomain":       ExitUsage,
		"subdomain_required":      ExitUsage,
		"invalid_local_addr":      ExitUsage,
		"machine_ambiguous":       ExitUsage,
		"ambiguous_port":          ExitUsage,
		"invalid_request":         ExitUsage,
		"invalid_tunnel_protocol": ExitUsage,
		"unsupported_proxy_type":  ExitUsage,
		"not_logged_in":           ExitNotLoggedIn,
		"invalid_machine":         ExitNotLoggedIn,
		"invalid_signature":       ExitNotLoggedIn,
		"machine_owner_inactive":  ExitNotLoggedIn,
		"login_expired":           ExitNotLoggedIn,
		"login_denied":            ExitNotLoggedIn,
		"no_active_subscription":  ExitQuota,
		"proxy_quota_exceeded":    ExitQuota,
		"domain_quota_exceeded":   ExitQuota,
		"port_quota_exceeded":     ExitQuota,
		"bandwidth_exceeded":      ExitQuota,
		"proxy_type_not_entitled": ExitQuota,
		"rootdomain_incompatible": ExitQuota,
		"no_rootdomain_available": ExitQuota,
		"cp_unreachable":          ExitUnreachable,
		"client_offline":          ExitClientOffline,
		"tunnel_not_found":        ExitError,
		"proxy_not_found":         ExitError,
		"anything_else":           ExitError,
	}
	for code, want := range cases {
		if got := ExitCodeFor(code); got != want {
			t.Errorf("ExitCodeFor(%q) = %d, want %d", code, got, want)
		}
	}
	if len(exitByCode)+3 != len(cases) {
		t.Fatalf("mapping has %d entries, test covers %d", len(exitByCode), len(cases)-3)
	}
}

func TestFromErrorClassifies(t *testing.T) {
	if got := FromError(&machineapi.APIError{Status: 403, Code: "proxy_quota_exceeded", Message: "满了"}); got.Code != "proxy_quota_exceeded" || got.ExitCode() != ExitQuota || !strings.Contains(got.Hint, "/billing") {
		t.Fatalf("api error = %+v", got)
	}
	if got := FromError(&machineapi.NetworkError{Err: errors.New("dial")}); got.Code != "cp_unreachable" || got.ExitCode() != ExitUnreachable {
		t.Fatalf("network error = %+v", got)
	}
	if got := FromError(errors.New("boom")); got.Code != "internal_error" || got.ExitCode() != ExitError {
		t.Fatalf("plain error = %+v", got)
	}
}

func TestPrinterJSONShapes(t *testing.T) {
	var out bytes.Buffer
	printer := Printer{JSON: true, Out: &out, Err: &out}
	if err := printer.Success(map[string]any{"public_url": "https://a.example/?x=1&y=2"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != `{"ok":true,"data":{"public_url":"https://a.example/?x=1&y=2"}}`+"\n" {
		t.Fatalf("success = %s", got)
	}

	out.Reset()
	code := printer.Fail(New("domain_quota_exceeded", "系统域名已用完"))
	if code != ExitQuota {
		t.Fatalf("exit = %d", code)
	}
	var decoded struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Hint    string `json:"hint"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.OK || decoded.Error.Code != "domain_quota_exceeded" || decoded.Error.Message != "系统域名已用完" || decoded.Error.Hint == "" {
		t.Fatalf("failure = %s", out.String())
	}
}

func TestPrinterHumanFailureGoesToStderr(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Printer{Out: &out, Err: &errOut}.Fail(New("client_offline", "离线"))
	if code != ExitClientOffline || out.Len() != 0 || !strings.Contains(errOut.String(), "client_offline") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}
