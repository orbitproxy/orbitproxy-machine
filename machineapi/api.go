package machineapi

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

type Usage struct {
	Used  int64 `json:"used"`
	Limit int64 `json:"limit"`
}

type Self struct {
	MachineID     string `json:"machine_id"`
	MachineKey    string `json:"machine_key"`
	Name          string `json:"name"`
	EdgeConnected bool   `json:"edge_connected"`
	PlanName      string `json:"plan_name"`
	Tunnels       *Usage `json:"tunnels"`
	SystemDomains *Usage `json:"system_domains"`
	BandwidthMbps *Usage `json:"bandwidth_mbps"`
}

type CreateTunnelRequest struct {
	Protocol      string `json:"protocol"`
	LocalPort     int    `json:"local_port"`
	LocalHost     string `json:"local_host,omitempty"`
	Subdomain     string `json:"subdomain,omitempty"`
	BandwidthMbps int    `json:"bandwidth_mbps,omitempty"`
}

type CreatedTunnel struct {
	ProxyID   string `json:"proxy_id"`
	PublicURL string `json:"public_url"`
	LocalAddr string `json:"local_addr"`
	Protocol  string `json:"protocol"`
	Created   bool   `json:"created"`
}

type Tunnel struct {
	ProxyID    string `json:"proxy_id"`
	PublicURL  string `json:"public_url"`
	LocalAddr  string `json:"local_addr"`
	Protocol   string `json:"protocol"`
	Status     string `json:"status"`
	CreatedVia string `json:"created_via"`
	CreatedAt  string `json:"created_at"`
}

func (client *Client) Self(ctx context.Context) (Self, error) {
	var out Self
	err := client.Do(ctx, http.MethodGet, "/v1/machine-api/self", nil, nil, &out)
	return out, err
}

func (client *Client) CreateTunnel(ctx context.Context, request CreateTunnelRequest) (CreatedTunnel, error) {
	var out CreatedTunnel
	err := client.Do(ctx, http.MethodPost, "/v1/machine-api/tunnels", nil, request, &out)
	return out, err
}

func (client *Client) ListTunnels(ctx context.Context) ([]Tunnel, error) {
	var out struct {
		Tunnels []Tunnel `json:"tunnels"`
	}
	err := client.Do(ctx, http.MethodGet, "/v1/machine-api/tunnels", nil, nil, &out)
	return out.Tunnels, err
}

func (client *Client) DeleteTunnel(ctx context.Context, proxyID string) error {
	return client.Do(ctx, http.MethodDelete, "/v1/machine-api/tunnels/"+url.PathEscape(proxyID), nil, nil, nil)
}

func (client *Client) Logout(ctx context.Context) error {
	return client.Do(ctx, http.MethodPost, "/v1/machine-api/logout", nil, struct{}{}, nil)
}

type DeviceStartRequest struct {
	PublicKey          string `json:"public_key"`
	PublicKeyAlgorithm string `json:"public_key_algorithm"`
	Hostname           string `json:"hostname"`
	MachineName        string `json:"machine_name,omitempty"`
	OS                 string `json:"os"`
	Arch               string `json:"arch"`
	ClientVersion      string `json:"client_version"`
	RestoreMachineKey  string `json:"restore_machine_key,omitempty"`
	Client             string `json:"client,omitempty"`
}

type DeviceStartResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

func StartDevice(ctx context.Context, httpClient *http.Client, baseURL string, request DeviceStartRequest) (DeviceStartResponse, error) {
	var out DeviceStartResponse
	err := postJSON(ctx, httpClient, baseURL+"/v1/cli-auth/device", request, &out)
	return out, err
}

// PollDeviceToken 用登录时生成的私钥签名取回授权结果；成功时 data 与 /v1/machines/register 相同。
func PollDeviceToken(ctx context.Context, httpClient *http.Client, baseURL, deviceCode string, privateKey ed25519.PrivateKey, now time.Time) (json.RawMessage, error) {
	timestamp := now.Unix()
	body := map[string]any{
		"device_code": deviceCode,
		"timestamp":   timestamp,
		"signature":   base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, DeviceSigningMessage(deviceCode, timestamp))),
	}
	var out json.RawMessage
	err := postJSON(ctx, httpClient, baseURL+"/v1/cli-auth/device/token", body, &out)
	return out, err
}
