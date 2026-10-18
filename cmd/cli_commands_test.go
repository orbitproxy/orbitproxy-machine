package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/machineapi"
	"github.com/orbitproxy/orbitproxy-machine/runlock"
)

// fakeCP 模拟控制面 /v1/cli-auth 与 /v1/machine-api 的最小行为。
type fakeCP struct {
	mu            sync.Mutex
	server        *httptest.Server
	tokenStatus   string // pending | approved | denied | expired
	startBodies   []machineapi.DeviceStartRequest
	tokenCalls    int
	edgeConnected bool
	tunnels       []machineapi.Tunnel
	createErr     string
	logoutStatus  int
	logoutCalls   int
	deleted       []string
}

func newFakeCP(t *testing.T) *fakeCP {
	t.Helper()
	cp := &fakeCP{tokenStatus: "pending", edgeConnected: true, logoutStatus: 200}
	cp.server = httptest.NewServer(http.HandlerFunc(cp.handle))
	t.Cleanup(cp.server.Close)
	return cp
}

func writeEnvelope(w http.ResponseWriter, status int, code string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{"data": data}
	if code != "" {
		body = map[string]any{"error": code, "message": code}
	}
	_ = json.NewEncoder(w).Encode(body)
}

func (cp *fakeCP) handle(w http.ResponseWriter, r *http.Request) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/cli-auth/device":
		var body machineapi.DeviceStartRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		cp.startBodies = append(cp.startBodies, body)
		writeEnvelope(w, 200, "", machineapi.DeviceStartResponse{
			DeviceCode:              "dc_1",
			UserCode:                "WDJB-MJHT",
			VerificationURI:         "https://console.test/authorize?target=workbuddy",
			VerificationURIComplete: "https://console.test/authorize?target=workbuddy&code=WDJB-MJHT",
			ExpiresIn:               600,
			Interval:                3,
		})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/cli-auth/device/token":
		cp.tokenCalls++
		switch cp.tokenStatus {
		case "approved":
			writeEnvelope(w, 200, "", map[string]any{
				"machine": map[string]any{"id": "m1", "machineKey": "mk_new", "type": "cli"},
				"edge":    map[string]any{"addr": "edge:443", "caCert": "ca"},
			})
		case "denied":
			writeEnvelope(w, 400, "access_denied", nil)
		case "expired":
			writeEnvelope(w, 400, "expired_token", nil)
		default:
			writeEnvelope(w, 400, "authorization_pending", nil)
		}
	case r.URL.Path == "/v1/machine-api/self":
		writeEnvelope(w, 200, "", machineapi.Self{MachineKey: r.Header.Get(machineapi.HeaderMachineKey), EdgeConnected: cp.edgeConnected})
	case r.Method == http.MethodPost && r.URL.Path == "/v1/machine-api/tunnels":
		if cp.createErr != "" {
			writeEnvelope(w, 403, cp.createErr, nil)
			return
		}
		var body machineapi.CreateTunnelRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		addr := net.JoinHostPort(body.LocalHost, strconv.Itoa(body.LocalPort))
		protocol := body.Protocol
		if protocol == "" {
			protocol = "https"
		}
		publicURL := "https://abc.example"
		if protocol == "tcp" {
			publicURL = "abc.example:20000"
		}
		writeEnvelope(w, 200, "", machineapi.CreatedTunnel{ProxyID: "prx_" + strconv.Itoa(body.LocalPort), PublicURL: publicURL, LocalAddr: addr, Protocol: protocol, Created: true})
	case r.Method == http.MethodGet && r.URL.Path == "/v1/machine-api/tunnels":
		writeEnvelope(w, 200, "", map[string]any{"tunnels": cp.tunnels})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/machine-api/tunnels/"):
		cp.deleted = append(cp.deleted, strings.TrimPrefix(r.URL.Path, "/v1/machine-api/tunnels/"))
		writeEnvelope(w, 200, "", nil)
	case r.URL.Path == "/v1/machine-api/logout":
		cp.logoutCalls++
		if cp.logoutStatus != 200 {
			writeEnvelope(w, cp.logoutStatus, "internal_error", nil)
			return
		}
		writeEnvelope(w, 200, "", map[string]string{"status": "logged_out"})
	default:
		writeEnvelope(w, 404, "not_found", nil)
	}
}

func testEnv(t *testing.T, baseURL string) (*cliEnv, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	buildmeta.SetSoftVersionForTest(t, "1.2.3")
	var out, errOut bytes.Buffer
	return &cliEnv{
		root:         t.TempDir(),
		out:          &out,
		errOut:       &errOut,
		in:           strings.NewReader(""),
		baseURL:      baseURL,
		http:         &http.Client{Timeout: 2 * time.Second},
		now:          time.Now,
		sleep:        func(time.Duration) {},
		hostname:     func() (string, error) { return "host-a", nil },
		onlineWait:   50 * time.Millisecond,
		endpointWait: 50 * time.Millisecond,
		pollEvery:    10 * time.Millisecond,
	}, &out, &errOut
}

// loggedInMachine 写入一台带真实密钥的机器配置并设为 default。
func loggedInMachine(t *testing.T, env *cliEnv, key string) string {
	t.Helper()
	publicKeyPEM, privateKeyPEM, err := generateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(env.root, key)
	if err := config.Save(dir, config.Config{MachineKey: key, Type: "cli", EdgeAddr: "edge:443", MachineCACert: "ca", PublicKeyPEM: publicKeyPEM, PrivateKeyPEM: privateKeyPEM}); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteKeyFile(config.DefaultPointerPath(env.root), key); err != nil {
		t.Fatal(err)
	}
	return dir
}

func exitCode(t *testing.T, printer cliout.Printer, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	if code, ok := exitCodeOf(runCLI(printer, func() error { return err })); ok {
		return code
	}
	t.Fatalf("unexpected error %v", err)
	return -1
}

func TestLoginNonTTYPrintsURLFirstAndExits(t *testing.T) {
	cp := newFakeCP(t)
	env, out, _ := testEnv(t, cp.server.URL)
	if err := config.WriteKeyFile(config.LastMachinePath(env.root), "mk_old"); err != nil {
		t.Fatal(err)
	}
	if err := runLogin(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 || lines[0] != "https://console.test/authorize?target=workbuddy&code=WDJB-MJHT" || !strings.Contains(lines[1], "WDJB-MJHT") {
		t.Fatalf("output = %q", out.String())
	}
	if len(cp.startBodies) != 1 || cp.startBodies[0].RestoreMachineKey != "mk_old" || cp.startBodies[0].Hostname != "host-a" || cp.startBodies[0].ClientVersion != "1.2.3" {
		t.Fatalf("start bodies = %+v", cp.startBodies)
	}
	if cp.tokenCalls != 0 {
		t.Fatalf("non-TTY login polled %d times", cp.tokenCalls)
	}
	pending, err := config.LoadPendingLogin(env.root)
	if err != nil || pending == nil || pending.DeviceCode != "dc_1" || pending.PrivateKeyPEM == "" {
		t.Fatalf("pending = %+v %v", pending, err)
	}
}

func TestLoginAlreadyLoggedIn(t *testing.T) {
	cp := newFakeCP(t)
	env, out, _ := testEnv(t, cp.server.URL)
	loggedInMachine(t, env, "mk_a")
	if err := runLogin(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if out.String() != "already logged in: mk_a\n" || len(cp.startBodies) != 0 {
		t.Fatalf("out = %q starts = %d", out.String(), len(cp.startBodies))
	}
}

func TestStatusStates(t *testing.T) {
	t.Run("logged in", func(t *testing.T) {
		cp := newFakeCP(t)
		env, out, _ := testEnv(t, cp.server.URL)
		loggedInMachine(t, env, "mk_a")
		if err := runStatus(context.Background(), env, "", false); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(out.String(), "logged_in yes\n") || cp.tokenCalls != 0 {
			t.Fatalf("out = %q", out.String())
		}
	})

	t.Run("no pending", func(t *testing.T) {
		cp := newFakeCP(t)
		env, out, errOut := testEnv(t, cp.server.URL)
		err := runStatus(context.Background(), env, "", false)
		if exitCode(t, env.printer(false), err) != cliout.ExitNotLoggedIn || out.String() != "logged_in no\n" || errOut.Len() != 0 {
			t.Fatalf("err=%v out=%q", err, out.String())
		}
	})

	for _, tc := range []struct {
		name        string
		tokenStatus string
		keepPending bool
	}{
		{"pending", "pending", true},
		{"denied", "denied", false},
		{"expired token", "expired", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := newFakeCP(t)
			env, out, _ := testEnv(t, cp.server.URL)
			if err := runLogin(context.Background(), env); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			cp.tokenStatus = tc.tokenStatus
			err := runStatus(context.Background(), env, "", false)
			if exitCode(t, env.printer(false), err) != cliout.ExitNotLoggedIn || !strings.HasPrefix(out.String(), "logged_in no\n") {
				t.Fatalf("err=%v out=%q", err, out.String())
			}
			pending, _ := config.LoadPendingLogin(env.root)
			if (pending != nil) != tc.keepPending {
				t.Fatalf("pending kept = %v", pending != nil)
			}
		})
	}

	t.Run("approved lands config and is idempotent", func(t *testing.T) {
		cp := newFakeCP(t)
		env, out, _ := testEnv(t, cp.server.URL)
		if err := config.WriteKeyFile(config.LastMachinePath(env.root), "mk_old"); err != nil {
			t.Fatal(err)
		}
		if err := runLogin(context.Background(), env); err != nil {
			t.Fatal(err)
		}
		cp.tokenStatus = "approved"
		out.Reset()
		if err := runStatus(context.Background(), env, "", false); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(out.String(), "logged_in yes\n") {
			t.Fatalf("out = %q", out.String())
		}
		cfg, err := config.Load(filepath.Join(env.root, "mk_new"))
		if err != nil || cfg.EdgeAddr != "edge:443" || cfg.PrivateKeyPEM == "" {
			t.Fatalf("cfg = %+v %v", cfg, err)
		}
		if pointer, _ := config.ReadKeyFile(config.DefaultPointerPath(env.root)); pointer != "mk_new" {
			t.Fatalf("default = %q", pointer)
		}
		for _, path := range []string{config.PendingLoginPath(env.root), config.LastMachinePath(env.root)} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("%s still exists", path)
			}
		}

		calls := cp.tokenCalls
		out.Reset()
		if err := runStatus(context.Background(), env, "", true); err != nil {
			t.Fatal(err)
		}
		if cp.tokenCalls != calls || !strings.Contains(out.String(), `"logged_in":true`) || !strings.Contains(out.String(), `"machine_key":"mk_new"`) {
			t.Fatalf("second status out=%q calls=%d", out.String(), cp.tokenCalls)
		}
	})

	t.Run("expired pending file is removed", func(t *testing.T) {
		cp := newFakeCP(t)
		env, out, _ := testEnv(t, cp.server.URL)
		if err := config.SavePendingLogin(env.root, config.PendingLogin{DeviceCode: "dc_1", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
			t.Fatal(err)
		}
		err := runStatus(context.Background(), env, "", false)
		if exitCode(t, env.printer(false), err) != cliout.ExitNotLoggedIn || out.String() != "logged_in no\n" || cp.tokenCalls != 0 {
			t.Fatalf("err=%v out=%q calls=%d", err, out.String(), cp.tokenCalls)
		}
		if pending, _ := config.LoadPendingLogin(env.root); pending != nil {
			t.Fatal("expired pending kept")
		}
	})

	t.Run("network error keeps pending", func(t *testing.T) {
		cp := newFakeCP(t)
		env, out, _ := testEnv(t, cp.server.URL)
		if err := runLogin(context.Background(), env); err != nil {
			t.Fatal(err)
		}
		env.baseURL = "http://127.0.0.1:1"
		out.Reset()
		err := runStatus(context.Background(), env, "", false)
		if exitCode(t, env.printer(false), err) != cliout.ExitNotLoggedIn || !strings.HasPrefix(out.String(), "logged_in no\n") {
			t.Fatalf("err=%v out=%q", err, out.String())
		}
		if pending, _ := config.LoadPendingLogin(env.root); pending == nil {
			t.Fatal("pending removed on network error")
		}
	})
}

func TestStatusAmbiguousMachinesIsLoggedOut(t *testing.T) {
	cp := newFakeCP(t)
	env, out, _ := testEnv(t, cp.server.URL)
	loggedInMachine(t, env, "mk_a")
	loggedInMachine(t, env, "mk_b")
	_ = os.Remove(config.DefaultPointerPath(env.root))
	err := runStatus(context.Background(), env, "", false)
	if exitCode(t, env.printer(false), err) != cliout.ExitNotLoggedIn || out.String() != "logged_in no\n" {
		t.Fatalf("err=%v out=%q", err, out.String())
	}
}

func TestLogoutCleansUpEvenWhenRemoteFails(t *testing.T) {
	cp := newFakeCP(t)
	cp.logoutStatus = 500
	env, out, errOut := testEnv(t, cp.server.URL)
	dir := loggedInMachine(t, env, "mk_a")
	if err := config.SavePendingLogin(env.root, config.PendingLogin{DeviceCode: "dc"}); err != nil {
		t.Fatal(err)
	}
	if err := runLogout(context.Background(), env, "", false); err != nil {
		t.Fatal(err)
	}
	if cp.logoutCalls != 1 || !strings.Contains(errOut.String(), "remote logout failed") || out.String() != "logged out: mk_a\n" {
		t.Fatalf("calls=%d out=%q err=%q", cp.logoutCalls, out.String(), errOut.String())
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("machine dir kept")
	}
	for _, path := range []string{config.DefaultPointerPath(env.root), config.PendingLoginPath(env.root)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s kept", path)
		}
	}
	if last, _ := config.ReadKeyFile(config.LastMachinePath(env.root)); last != "mk_a" {
		t.Fatalf("last-machine = %q", last)
	}
}

func TestLogoutNotLoggedInIsZero(t *testing.T) {
	cp := newFakeCP(t)
	env, out, _ := testEnv(t, cp.server.URL)
	if err := runLogout(context.Background(), env, "", false); err != nil {
		t.Fatal(err)
	}
	if out.String() != "not logged in\n" || cp.logoutCalls != 0 {
		t.Fatalf("out=%q calls=%d", out.String(), cp.logoutCalls)
	}
}

func TestProxyAdd(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)

	t.Run("creates without starting the client", func(t *testing.T) {
		cp := newFakeCP(t)
		env, out, _ := testEnv(t, cp.server.URL)
		loggedInMachine(t, env, "mk_a")
		env.spawn = func(string, []string, string) (int, error) { t.Fatal("proxy add must not spawn"); return 0, nil }
		if err := runProxyAdd(context.Background(), env, proxyAddOptions{proxyType: proxyTypeAPIGateway, protocol: "https", port: port, host: defaultShareHost, json: true}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"public_url":"https://abc.example"`) || strings.Contains(out.String(), "local_not_listening") {
			t.Fatalf("out = %s", out.String())
		}
	})

	t.Run("warns when local port is closed", func(t *testing.T) {
		cp := newFakeCP(t)
		env, out, _ := testEnv(t, cp.server.URL)
		loggedInMachine(t, env, "mk_a")
		if err := runProxyAdd(context.Background(), env, proxyAddOptions{proxyType: proxyTypeAPIGateway, protocol: "https", port: "1", host: defaultShareHost, json: true}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"warnings":["local_not_listening"]`) {
			t.Fatalf("out = %s", out.String())
		}
	})

	t.Run("tcp", func(t *testing.T) {
		cp := newFakeCP(t)
		env, out, _ := testEnv(t, cp.server.URL)
		loggedInMachine(t, env, "mk_a")
		if err := runProxyAdd(context.Background(), env, proxyAddOptions{proxyType: proxyTypeAPIGateway, protocol: "tcp", port: port, host: defaultShareHost, json: true}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"protocol":"tcp"`) || !strings.Contains(out.String(), `"public_url":"abc.example:20000"`) {
			t.Fatalf("out = %s", out.String())
		}
	})

	t.Run("quota error passes through with hint", func(t *testing.T) {
		cp := newFakeCP(t)
		cp.createErr = "domain_quota_exceeded"
		env, out, _ := testEnv(t, cp.server.URL)
		loggedInMachine(t, env, "mk_a")
		err := runProxyAdd(context.Background(), env, proxyAddOptions{proxyType: proxyTypeAPIGateway, protocol: "https", port: port, host: defaultShareHost, json: true})
		if exitCode(t, env.printer(true), err) != cliout.ExitQuota || !strings.Contains(out.String(), "/billing") {
			t.Fatalf("err=%v out=%s", err, out.String())
		}
	})

	t.Run("rejects bad arguments", func(t *testing.T) {
		cp := newFakeCP(t)
		env, _, _ := testEnv(t, cp.server.URL)
		loggedInMachine(t, env, "mk_a")
		cases := []proxyAddOptions{
			{protocol: "https", port: port},
			{proxyType: "mcp_connector", protocol: "https", port: port},
			{proxyType: proxyTypeAPIGateway, protocol: "http", port: port},
			{proxyType: proxyTypeAPIGateway, protocol: "tcp", port: port, subdomain: "abc"},
			{proxyType: proxyTypeAPIGateway, protocol: "https", port: "abc"},
		}
		for _, opts := range cases {
			opts.host = defaultShareHost
			if code := exitCode(t, env.printer(false), runProxyAdd(context.Background(), env, opts)); code != cliout.ExitUsage {
				t.Fatalf("opts %+v code = %d", opts, code)
			}
		}
		err := runProxyAdd(context.Background(), env, proxyAddOptions{proxyType: proxyTypeAPIGateway, protocol: "https", port: "3000", host: defaultShareHost, machineKey: "missing"})
		if exitCode(t, env.printer(false), err) != cliout.ExitNotLoggedIn {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestRunBackgroundReusesHolderAndWaits(t *testing.T) {
	cp := newFakeCP(t)
	env, out, _ := testEnv(t, cp.server.URL)
	dir := loggedInMachine(t, env, "mk_a")
	lock, err := runlock.Acquire(dir, runlock.ModeService)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	spawned := 0
	env.spawn = func(string, []string, string) (int, error) { spawned++; return 1, nil }
	runMachineKeyFlag = "mk_a"
	t.Cleanup(func() { runMachineKeyFlag = "" })
	if err := runInBackground(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if spawned != 0 || !strings.Contains(out.String(), "already running (service") {
		t.Fatalf("spawned=%d out=%s", spawned, out.String())
	}
}

func TestCloseByIDPortAndAmbiguous(t *testing.T) {
	cp := newFakeCP(t)
	cp.tunnels = []machineapi.Tunnel{
		{ProxyID: "prx_a", LocalAddr: "127.0.0.1:3000"},
		{ProxyID: "prx_b", LocalAddr: "127.0.0.1:4000"},
		{ProxyID: "prx_c", LocalAddr: "192.168.1.5:4000"},
	}
	env, out, _ := testEnv(t, cp.server.URL)
	loggedInMachine(t, env, "mk_a")

	if err := runClose(context.Background(), env, "prx_x", "", false); err != nil {
		t.Fatal(err)
	}
	if err := runClose(context.Background(), env, "3000", "", true); err != nil {
		t.Fatal(err)
	}
	if len(cp.deleted) != 2 || cp.deleted[0] != "prx_x" || cp.deleted[1] != "prx_a" {
		t.Fatalf("deleted = %v", cp.deleted)
	}

	out.Reset()
	err := runClose(context.Background(), env, "4000", "", true)
	if exitCode(t, env.printer(true), err) != cliout.ExitUsage || !strings.Contains(out.String(), "prx_b") || !strings.Contains(out.String(), "prx_c") {
		t.Fatalf("err=%v out=%s", err, out.String())
	}
	err = runClose(context.Background(), env, "5000", "", true)
	if exitCode(t, env.printer(true), err) != cliout.ExitError || !strings.Contains(out.String(), "tunnel_not_found") {
		t.Fatalf("err=%v out=%s", err, out.String())
	}
	if len(cp.deleted) != 2 {
		t.Fatalf("deleted = %v", cp.deleted)
	}
}

func TestListJSON(t *testing.T) {
	cp := newFakeCP(t)
	env, out, _ := testEnv(t, cp.server.URL)
	loggedInMachine(t, env, "mk_a")
	if err := runList(context.Background(), env, "", true); err != nil {
		t.Fatal(err)
	}
	if out.String() != `{"ok":true,"data":{"tunnels":[]}}`+"\n" {
		t.Fatalf("out = %s", out.String())
	}
}

func TestStopOnlyBackground(t *testing.T) {
	cp := newFakeCP(t)
	env, out, _ := testEnv(t, cp.server.URL)
	dir := loggedInMachine(t, env, "mk_a")
	if err := runStop(env, ""); err != nil || out.String() != "not running\n" {
		t.Fatalf("err=%v out=%q", err, out.String())
	}
	lock, err := runlock.Acquire(dir, runlock.ModeService)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	if code := exitCode(t, env.printer(false), runStop(env, "")); code != cliout.ExitError {
		t.Fatalf("code = %d", code)
	}
}

func TestVersionJSON(t *testing.T) {
	buildmeta.SetSoftVersionForTest(t, "1.2.3")
	var out bytes.Buffer
	if err := printVersion(cliout.Printer{JSON: true, Out: &out, Err: io.Discard}); err != nil {
		t.Fatal(err)
	}
	if out.String() != `{"ok":true,"data":{"version":"1.2.3"}}`+"\n" {
		t.Fatalf("out = %s", out.String())
	}
}

func TestEndpointsSnapshotFromStatus(t *testing.T) {
	snapshot := endpointsSnapshot(nil, time.Unix(1, 0))
	if snapshot.Endpoints == nil || len(snapshot.Endpoints) != 0 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}
