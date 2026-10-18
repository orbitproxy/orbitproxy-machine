// Package machineapi 调用控制面 /v1/machine-api（机器私钥签名）与 /v1/cli-auth（设备授权）。
package machineapi

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	HeaderMachineKey = "X-OrbitProxy-Machine-Key"
	HeaderTimestamp  = "X-OrbitProxy-Timestamp"
	HeaderNonce      = "X-OrbitProxy-Nonce"
	HeaderSignature  = "X-OrbitProxy-Signature"

	signatureVersion = "OP-MACHINE-V1"
)

// SignedRequest 是参与签名的请求要素，必须与控制面 pkg/machinesig 逐字节一致。
type SignedRequest struct {
	Method     string
	Path       string
	Query      url.Values
	Body       []byte
	Timestamp  string
	Nonce      string
	MachineKey string
}

func Canonical(request SignedRequest) string {
	sum := sha256.Sum256(request.Body)
	return strings.Join([]string{
		signatureVersion,
		request.Method,
		request.Path,
		canonicalQuery(request.Query),
		hex.EncodeToString(sum[:]),
		request.Timestamp,
		request.Nonce,
		request.MachineKey,
	}, "\n")
}

func canonicalQuery(query url.Values) string {
	if len(query) == 0 {
		return ""
	}
	pairs := make([]string, 0, len(query))
	for key, values := range query {
		for _, value := range values {
			pairs = append(pairs, key+"="+value)
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "&")
}

func Sign(privateKey ed25519.PrivateKey, request SignedRequest) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(Canonical(request))))
}

func newNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// DeviceSigningMessage 是 /v1/cli-auth/device/token 的签名原文。
func DeviceSigningMessage(deviceCode string, timestamp int64) []byte {
	return []byte(deviceCode + "\n" + strconv.FormatInt(timestamp, 10))
}
