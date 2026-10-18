package machineapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func vectorKey() ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	return ed25519.NewKeyFromSeed(seed)
}

// 固定向量与控制面 pkg/machinesig/machinesig_test.go 共用。
func TestSignFixedVector(t *testing.T) {
	request := SignedRequest{
		Method:     "POST",
		Path:       "/v1/machine-api/tunnels",
		Query:      url.Values{"b": {"2"}, "a": {"1"}},
		Body:       []byte(`{"protocol":"https","local_port":3000}`),
		Timestamp:  "1760000000",
		Nonce:      "bm9uY2U",
		MachineKey: "mk_vector",
	}
	const want = "vMv37nJ1XFGlT3ag5BRFhRO3UwoMJN/aL9QeQoNUKwEKplFtKmwkQaxFxshBs4rZZ+lDGBa3vJldUNuS3snBAA=="
	if got := Sign(vectorKey(), request); got != want {
		t.Fatalf("signature = %s, want %s", got, want)
	}
	lines := strings.Split(Canonical(request), "\n")
	if len(lines) != 8 || lines[3] != "a=1&b=2" || lines[4] != "169ea2e8a4c199bfe8585190ca17e76e7ccf7ec026c30d1ebc04c31bc82c9386" {
		t.Fatalf("canonical lines = %q", lines)
	}
}

func TestCanonicalEmptyQueryAndBody(t *testing.T) {
	lines := strings.Split(Canonical(SignedRequest{Method: "GET", Path: "/v1/machine-api/self", Timestamp: "1", Nonce: "n", MachineKey: "mk"}), "\n")
	if lines[3] != "" || lines[4] != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("lines = %q", lines)
	}
}

func TestNewNonceIs16BytesBase64URL(t *testing.T) {
	nonce, err := newNonce()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(nonce)
	if err != nil || len(raw) != 16 {
		t.Fatalf("nonce %q decoded %d bytes, err %v", nonce, len(raw), err)
	}
}

func TestDeviceSigningMessage(t *testing.T) {
	if got := string(DeviceSigningMessage("dc_1", 1760000000)); got != "dc_1\n1760000000" {
		t.Fatalf("message = %q", got)
	}
}
