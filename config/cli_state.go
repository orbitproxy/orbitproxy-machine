package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultPointerFile = "default"
	lastMachineFile    = "last-machine"
	pendingLoginFile   = "pending-login.json"
	endpointsFile      = "endpoints.json"
)

// DefaultPointerPath 是 <root>/default，内容为 login 写入的 machine_key。
func DefaultPointerPath(root string) string { return filepath.Join(root, defaultPointerFile) }

// LastMachinePath 是 <root>/last-machine，logout 留下的 machine_key，只用于下次 login 申请恢复。
func LastMachinePath(root string) string { return filepath.Join(root, lastMachineFile) }

// PendingLoginPath 是 <root>/pending-login.json，login 与 status 之间传递设备授权会话。
func PendingLoginPath(root string) string { return filepath.Join(root, pendingLoginFile) }

// EndpointsPath 是 <machineDir>/endpoints.json，run 进程写入当前端点快照。
func EndpointsPath(machineDir string) string { return filepath.Join(machineDir, endpointsFile) }

func ReadKeyFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func WriteKeyFile(path, machineKey string) error {
	return WriteFileAtomic(path, []byte(machineKey), 0o600)
}

func RemoveIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// WriteFileAtomic 先写临时文件再 rename，读方不会看到写了一半的内容。
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// PendingLogin 是一次未完成的设备授权；私钥只在取回结果时用来签名。
type PendingLogin struct {
	DeviceCode              string    `json:"device_code"`
	UserCode                string    `json:"user_code"`
	VerificationURIComplete string    `json:"verification_uri_complete"`
	PrivateKeyPEM           string    `json:"private_key_pem"`
	PublicKeyPEM            string    `json:"public_key_pem"`
	ExpiresAt               time.Time `json:"expires_at"`
	Interval                int       `json:"interval"`
}

func LoadPendingLogin(root string) (*PendingLogin, error) {
	data, err := os.ReadFile(PendingLoginPath(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var pending PendingLogin
	if err := json.Unmarshal(data, &pending); err != nil {
		return nil, fmt.Errorf("parse %s: %w", PendingLoginPath(root), err)
	}
	return &pending, nil
}

func SavePendingLogin(root string, pending PendingLogin) error {
	data, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(PendingLoginPath(root), data, 0o600)
}

// EndpointsSnapshot 是 endpoints.json 的内容。
type EndpointsSnapshot struct {
	UpdatedAt time.Time       `json:"updated_at"`
	Endpoints []EndpointEntry `json:"endpoints"`
}

type EndpointEntry struct {
	EndpointID string          `json:"endpoint_id"`
	LocalAddr  string          `json:"local_addr"`
	Proxies    []EndpointProxy `json:"proxies"`
}

type EndpointProxy struct {
	ProxyID string `json:"proxy_id"`
	PubHost string `json:"pub_host"`
	PubPort int    `json:"pub_port"`
}

func SaveEndpoints(machineDir string, snapshot EndpointsSnapshot) error {
	if snapshot.Endpoints == nil {
		snapshot.Endpoints = []EndpointEntry{}
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return WriteFileAtomic(EndpointsPath(machineDir), data, 0o600)
}

func LoadEndpoints(machineDir string) (*EndpointsSnapshot, error) {
	data, err := os.ReadFile(EndpointsPath(machineDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snapshot EndpointsSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (snapshot *EndpointsSnapshot) HasProxy(proxyID string) bool {
	if snapshot == nil {
		return false
	}
	for _, endpoint := range snapshot.Endpoints {
		for _, proxy := range endpoint.Proxies {
			if proxy.ProxyID == proxyID {
				return true
			}
		}
	}
	return false
}

var (
	ErrNoMachine        = errors.New("no machine config")
	ErrMachineAmbiguous = errors.New("multiple machines")
)

// SelectMachineKey：--machine-key > default 指针 > 唯一一台；多台且无指针返回 ErrMachineAmbiguous。
func SelectMachineKey(root, explicit string) (string, error) {
	if explicit != "" {
		dir, err := MachineDir(root, explicit)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(Path(dir)); err != nil {
			return "", fmt.Errorf("%w: %s", ErrNoMachine, explicit)
		}
		return explicit, nil
	}
	pointer, err := ReadKeyFile(DefaultPointerPath(root))
	if err != nil {
		return "", err
	}
	if pointer != "" {
		if _, err := os.Stat(Path(filepath.Join(root, pointer))); err == nil {
			return pointer, nil
		}
	}
	keys, err := ListMachineKeys(root)
	if err != nil || len(keys) == 0 {
		return "", ErrNoMachine
	}
	if len(keys) > 1 {
		return "", fmt.Errorf("%w under %s (%s); pass --machine-key", ErrMachineAmbiguous, root, strings.Join(keys, ", "))
	}
	return keys[0], nil
}
