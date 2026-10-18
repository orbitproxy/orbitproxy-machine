package machineapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	DefaultTimeout = 5 * time.Second
	LogoutTimeout  = 20 * time.Second
	maxResponse    = 4 << 20
)

// APIError 是控制面返回的业务错误，Code 即 apierr.code。
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (err *APIError) Error() string {
	if err.Message != "" {
		return err.Message
	}
	return fmt.Sprintf("control-plane returned %d %s", err.Status, err.Code)
}

// NetworkError 表示控制面不可达或响应无法解析。
type NetworkError struct{ Err error }

func (err *NetworkError) Error() string { return "control-plane unreachable: " + err.Err.Error() }
func (err *NetworkError) Unwrap() error { return err.Err }

type envelope struct {
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
	Message string          `json:"message"`
}

// Client 用机器私钥签名请求。
type Client struct {
	BaseURL    string
	MachineKey string
	PrivateKey ed25519.PrivateKey
	HTTP       *http.Client
	Now        func() time.Time
}

func NewClient(baseURL, machineKey, privateKeyPEM string) (*Client, error) {
	privateKey, err := ParsePrivateKeyPEM(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	return &Client{
		BaseURL:    baseURL,
		MachineKey: machineKey,
		PrivateKey: privateKey,
		HTTP:       &http.Client{Timeout: DefaultTimeout},
	}, nil
}

func ParsePrivateKeyPEM(privateKeyPEM string) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("machine private key is not valid PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse machine private key: %w", err)
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("machine private key is not ed25519")
	}
	return key, nil
}

func (client *Client) now() time.Time {
	if client.Now != nil {
		return client.Now()
	}
	return time.Now()
}

// Do 发送签名请求；body 为 nil 时不带请求体，out 为 nil 时忽略 data。
func (client *Client) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var payload []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = encoded
	}
	nonce, err := newNonce()
	if err != nil {
		return err
	}
	timestamp := strconv.FormatInt(client.now().Unix(), 10)
	signature := Sign(client.PrivateKey, SignedRequest{
		Method:     method,
		Path:       path,
		Query:      query,
		Body:       payload,
		Timestamp:  timestamp,
		Nonce:      nonce,
		MachineKey: client.MachineKey,
	})

	target := client.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set(HeaderMachineKey, client.MachineKey)
	request.Header.Set(HeaderTimestamp, timestamp)
	request.Header.Set(HeaderNonce, nonce)
	request.Header.Set(HeaderSignature, signature)
	return send(client.HTTP, request, out)
}

func send(httpClient *http.Client, request *http.Request, out any) error {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Language", "zh-CN")
	response, err := httpClient.Do(request)
	if err != nil {
		return &NetworkError{Err: err}
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponse))
	if err != nil {
		return &NetworkError{Err: err}
	}
	var decoded envelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return &NetworkError{Err: fmt.Errorf("decode response (%d): %w", response.StatusCode, err)}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || decoded.Error != "" {
		return &APIError{Status: response.StatusCode, Code: decoded.Error, Message: decoded.Message}
	}
	if out == nil || len(decoded.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(decoded.Data, out); err != nil {
		return &NetworkError{Err: fmt.Errorf("decode data: %w", err)}
	}
	return nil
}

func postJSON(ctx context.Context, httpClient *http.Client, target string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	return send(httpClient, request, out)
}

// IsCode 判断 err 是否为指定 code 的控制面错误。
func IsCode(err error, code string) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}
