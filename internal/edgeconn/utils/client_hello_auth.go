package utils

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/orbitlink/orbitlink-machine/internal/edgeconn/msg"
)

const ClientHelloCanonicalVersion = "orbitlink-edge-client-hello/v1"

// ClientHelloCanonicalString 构建 ClientHello 签名用的 canonical 字符串。
func ClientHelloCanonicalString(machineID string, timestamp int64, nonce, softVersion string) string {
	lines := []string{
		ClientHelloCanonicalVersion,
		strings.TrimSpace(machineID),
		strconv.FormatInt(timestamp, 10),
		strings.TrimSpace(nonce),
		strings.TrimSpace(softVersion),
	}
	return strings.Join(lines, "\n") + "\n"
}

// NewClientHello 构造待签名的 ClientHello。
func NewClientHello(machineID, softVersion string) (msg.ClientHello, error) {
	nonce, err := randomNonce()
	if err != nil {
		return msg.ClientHello{}, err
	}

	return msg.ClientHello{
		MachineID:   strings.TrimSpace(machineID),
		Timestamp:   time.Now().Unix(),
		Nonce:       nonce,
		SoftVersion: strings.TrimSpace(softVersion),
	}, nil
}

// SignClientHello 用 machine 私钥为 ClientHello 签名。
func SignClientHello(privateKeyPEM string, hello msg.ClientHello) (msg.ClientHello, error) {
	privateKey, err := parseEd25519PrivateKey(privateKeyPEM)
	if err != nil {
		return msg.ClientHello{}, err
	}

	canonical := ClientHelloCanonicalString(
		hello.MachineID,
		hello.Timestamp,
		hello.Nonce,
		hello.SoftVersion,
	)
	signature := ed25519.Sign(privateKey, []byte(canonical))
	hello.AuthSignature = base64.StdEncoding.EncodeToString(signature)
	return hello, nil
}

func parseEd25519PrivateKey(privateKeyPEM string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(privateKeyPEM)))
	if block == nil {
		return nil, fmt.Errorf("private key must be a valid PEM encoded key")
	}

	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	privateKey, ok := keyAny.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key must be ed25519")
	}
	return privateKey, nil
}

func randomNonce() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate client hello nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
