package machineapi

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func vectorPEM(t *testing.T) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(vectorKey())
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestClientSignsRequestAndDecodesData(t *testing.T) {
	publicKey := vectorKey().Public().(ed25519.PublicKey)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		request := SignedRequest{
			Method:     r.Method,
			Path:       r.URL.Path,
			Query:      r.URL.Query(),
			Body:       body,
			Timestamp:  r.Header.Get(HeaderTimestamp),
			Nonce:      r.Header.Get(HeaderNonce),
			MachineKey: r.Header.Get(HeaderMachineKey),
		}
		signature, err := base64.StdEncoding.DecodeString(r.Header.Get(HeaderSignature))
		if err != nil || !ed25519.Verify(publicKey, []byte(Canonical(request)), signature) {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"invalid_signature","message":"bad"}`))
			return
		}
		if request.Timestamp != "1760000000" || request.MachineKey != "mk_1" || r.Header.Get("Accept-Language") != "zh-CN" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"unexpected","message":"headers"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"proxy_id":"prx_1","public_url":"https://a.example","local_addr":"127.0.0.1:3000","protocol":"https","created":true}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "mk_1", vectorPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	client.Now = func() time.Time { return time.Unix(1760000000, 0) }
	created, err := client.CreateTunnel(context.Background(), CreateTunnelRequest{Protocol: "https", LocalPort: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if created.ProxyID != "prx_1" || !created.Created {
		t.Fatalf("created = %+v", created)
	}
}

func TestClientMapsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":"domain_quota_exceeded","message":"系统域名已用完"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "mk_1", vectorPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Self(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 403 || apiErr.Code != "domain_quota_exceeded" || apiErr.Message != "系统域名已用完" {
		t.Fatalf("err = %#v", err)
	}
	if !IsCode(err, "domain_quota_exceeded") {
		t.Fatal("IsCode false")
	}
}

func TestClientMapsNetworkError(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:1", "mk_1", vectorPEM(t))
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Self(context.Background())
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Fatalf("err = %#v", err)
	}
}

func TestPollDeviceTokenSignsDeviceCode(t *testing.T) {
	publicKey := vectorKey().Public().(ed25519.PublicKey)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			DeviceCode string `json:"device_code"`
			Timestamp  int64  `json:"timestamp"`
			Signature  string `json:"signature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(400)
			return
		}
		signature, _ := base64.StdEncoding.DecodeString(body.Signature)
		if r.URL.Path != "/v1/cli-auth/device/token" || !ed25519.Verify(publicKey, DeviceSigningMessage(body.DeviceCode, body.Timestamp), signature) {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"invalid_signature"}`))
			return
		}
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
	}))
	defer server.Close()
	_, err := PollDeviceToken(context.Background(), server.Client(), server.URL, "dc_1", vectorKey(), time.Unix(1760000000, 0))
	if !IsCode(err, "authorization_pending") {
		t.Fatalf("err = %v", err)
	}
}
