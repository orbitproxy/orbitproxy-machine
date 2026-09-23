package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/config"
	"github.com/orbitproxy/orbitproxy-machine/httpclient"
	"github.com/orbitproxy/orbitproxy-machine/logger"
	"github.com/spf13/cobra"
)

const publicKeyAlgorithm = "ed25519"

var authtokenCmd = &cobra.Command{
	Use:   "authtoken <token>",
	Short: "Register this machine with an authtoken",
	Long:  "Generate a local machine identity, register with control-plane, and persist machine config under <workdir>/<machineKey>/.",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthtoken,
}

func init() {
	authtokenCmd.Flags().String("name", "", "optional machine display name")
	authtokenCmd.Flags().String("machine-key", "", "restore an existing machine by machine key")
	authtokenCmd.Flags().Bool("force", false, "overwrite existing local config")
	rootCmd.AddCommand(authtokenCmd)
}

type registerOptions struct {
	authToken   string
	machineKey  string
	workDir     string
	machineName string
	hostname    string
	force       bool
	httpClient  *http.Client
}

type registerResult struct {
	workDir              string
	configPath           string
	machineKey           string
	edgeAddr             string
	tlsServerName        string
	publicKeyFingerprint string
}

type registerRequest struct {
	AuthToken          string `json:"authtoken"`
	MachineKey         string `json:"machineKey,omitempty"`
	Type               string `json:"type,omitempty"`
	MachineName        string `json:"machineName,omitempty"`
	Hostname           string `json:"hostname,omitempty"`
	PublicKeyAlgorithm string `json:"publicKeyAlgorithm"`
	PublicKey          string `json:"publicKey"`
	httpclient.Base
}

type registerResponseData struct {
	Machine struct {
		ID                   string `json:"id"`
		MachineKey           string `json:"machineKey"`
		Type                 string `json:"type"`
		PublicKeyFingerprint string `json:"publicKeyFingerprint"`
	} `json:"machine"`
	Edge struct {
		Addr   string `json:"addr"`
		CACert string `json:"caCert"`
	} `json:"edge"`
}

func runAuthtoken(cmd *cobra.Command, args []string) error {
	root, err := config.ResolveRoot(workDirFlag)
	if err != nil {
		return err
	}

	machineName, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	machineKey, err := cmd.Flags().GetString("machine-key")
	if err != nil {
		return err
	}
	force, err := cmd.Flags().GetBool("force")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
	defer cancel()

	result, err := registerMachine(ctx, registerOptions{
		authToken:   args[0],
		machineKey:  machineKey,
		workDir:     root,
		machineName: machineName,
		force:       force,
	})
	if err != nil {
		return err
	}

	pairs := registrationDetailPairs(result)
	log := logger.New(logger.Options{
		LogDir: config.LogsDir(result.workDir),
	})
	attrs := make([]any, 0, len(pairs)*2)
	for _, pair := range pairs {
		attrs = append(attrs, pair[0], pair[1])
	}
	log.Info("machine registration complete", attrs...)

	fmt.Println("machine registration complete")
	fmt.Println()
	for _, pair := range pairs {
		if pair[1] == "" {
			continue
		}
		fmt.Printf("%s: %s\n", pair[0], pair[1])
	}
	fmt.Println()
	return nil
}

func registerMachine(ctx context.Context, opts registerOptions) (*registerResult, error) {
	root, err := config.ResolveRoot(opts.workDir)
	if err != nil {
		return nil, err
	}

	requestedKey := strings.TrimSpace(opts.machineKey)
	if requestedKey != "" && !opts.force {
		machineDir, err := config.MachineDir(root, requestedKey)
		if err != nil {
			return nil, err
		}
		configPath := config.Path(machineDir)
		if _, err := os.Stat(configPath); err == nil {
			return nil, fmt.Errorf("machine config already exists at %s; use --force to overwrite", configPath)
		} else if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect machine config %s: %w", configPath, err)
		}
	}

	authToken := strings.TrimSpace(opts.authToken)
	if authToken == "" {
		return nil, fmt.Errorf("authtoken is required")
	}

	hostname := strings.TrimSpace(opts.hostname)
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	machineName := strings.TrimSpace(opts.machineName)
	if machineName == "" {
		machineName = hostname
	}

	publicKeyPEM, privateKeyPEM, err := generateKeyPair()
	if err != nil {
		return nil, err
	}

	if !buildmeta.IsClientSemver(buildmeta.SoftVersion) {
		return nil, fmt.Errorf("client version is required; inject buildmeta.SoftVersion via -ldflags as semver, got %q", buildmeta.SoftVersion)
	}

	reqBody := registerRequest{
		AuthToken:          authToken,
		MachineKey:         requestedKey,
		MachineName:        machineName,
		Hostname:           hostname,
		PublicKeyAlgorithm: publicKeyAlgorithm,
		PublicKey:          publicKeyPEM,
		Base:               httpclient.Base{}.Resolve(buildmeta.SoftVersion),
	}
	if requestedKey == "" {
		reqBody.Type = "cli"
	}

	registerURL := buildmeta.ControlPlaneURL + "/v1/machines/register"
	if opts.httpClient != nil {
		httpclient.SetOverride(opts.httpClient)
		defer httpclient.SetOverride(nil)
	}

	respEnvelope, statusCode, err := httpclient.PostJSON[registerRequest, httpclient.Response[registerResponseData]](
		httpclient.JSON(), ctx, registerURL, reqBody,
	)
	if err != nil {
		return nil, fmt.Errorf("register machine: %w", err)
	}
	if statusCode < 200 || statusCode >= 300 {
		if msg := strings.TrimSpace(respEnvelope.Message); msg != "" {
			return nil, fmt.Errorf("register machine failed: %s", msg)
		}
		if code := strings.TrimSpace(respEnvelope.Error); code != "" {
			return nil, fmt.Errorf("register machine failed: %s", code)
		}
		return nil, fmt.Errorf("register machine failed: control-plane returned %d", statusCode)
	}

	respData := respEnvelope.Data
	if strings.TrimSpace(respData.Machine.MachineKey) == "" ||
		strings.TrimSpace(respData.Edge.Addr) == "" ||
		strings.TrimSpace(respData.Edge.CACert) == "" {
		return nil, fmt.Errorf("register machine failed: control-plane response is missing machine registration data")
	}

	machineDir, err := config.MachineDir(root, respData.Machine.MachineKey)
	if err != nil {
		return nil, err
	}
	configPath := config.Path(machineDir)
	if !opts.force {
		if _, err := os.Stat(configPath); err == nil {
			return nil, fmt.Errorf("machine config already exists at %s; use --force to overwrite", configPath)
		} else if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect machine config %s: %w", configPath, err)
		}
	}

	machineCfg := config.Config{
		MachineKey:    respData.Machine.MachineKey,
		Type:          respData.Machine.Type,
		EdgeAddr:      respData.Edge.Addr,
		MachineCACert: respData.Edge.CACert,
		PublicKeyPEM:  publicKeyPEM,
		PrivateKeyPEM: privateKeyPEM,
	}
	if machineCfg.Type == "" {
		machineCfg.Type = "cli"
	}
	if err := config.Save(machineDir, machineCfg); err != nil {
		return nil, err
	}

	return &registerResult{
		workDir:              machineDir,
		configPath:           configPath,
		machineKey:           machineCfg.MachineKey,
		edgeAddr:             machineCfg.EdgeAddr,
		tlsServerName:        machineCfg.TLSServerName,
		publicKeyFingerprint: strings.TrimSpace(respData.Machine.PublicKeyFingerprint),
	}, nil
}

func generateKeyPair() (string, string, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate ed25519 key pair: %w", err)
	}

	publicDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal public key: %w", err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal private key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
		string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
		nil
}

func registrationDetailPairs(result *registerResult) [][2]string {
	pairs := [][2]string{
		{"machine_key", result.machineKey},
		{"config", result.configPath},
		{"workdir", result.workDir},
		{"edge", result.edgeAddr},
	}
	if result.tlsServerName != "" {
		pairs = append(pairs, [2]string{"tls_server_name", result.tlsServerName})
	}
	if result.publicKeyFingerprint != "" {
		pairs = append(pairs, [2]string{"public_key_fingerprint", result.publicKeyFingerprint})
	}
	return pairs
}
