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

	"github.com/orbitlink/orbitlink-machine/internal/buildmeta"
	"github.com/orbitlink/orbitlink-machine/internal/config"
	"github.com/orbitlink/orbitlink-machine/internal/httpclient"
	"github.com/orbitlink/orbitlink-machine/internal/logger"
	"github.com/spf13/cobra"
)

const publicKeyAlgorithm = "ed25519"

var authtokenCmd = &cobra.Command{
	Use:   "authtoken <token>",
	Short: "Register this machine with an authtoken",
	Long:  "Generate a local machine identity, register with control-plane, and persist machine config under the workdir.",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthtoken,
}

func init() {
	authtokenCmd.Flags().String("machine-id", "", "reuse an existing local machine ID")
	authtokenCmd.Flags().String("name", "", "optional machine display name")
	authtokenCmd.Flags().Bool("force", false, "overwrite existing local config")
	rootCmd.AddCommand(authtokenCmd)
}

type registerOptions struct {
	authToken   string
	workDir     string
	machineID   string
	machineName string
	hostname    string
	force       bool
	httpClient  *http.Client
}

type registerResult struct {
	workDir              string
	configPath           string
	machineID            string
	edgeAddr             string
	tlsServerName        string
	publicKeyFingerprint string
}

type registerRequest struct {
	AuthToken          string `json:"authtoken"`
	MachineID          string `json:"machineId,omitempty"`
	MachineName        string `json:"machineName,omitempty"`
	Hostname           string `json:"hostname,omitempty"`
	PublicKeyAlgorithm string `json:"publicKeyAlgorithm"`
	PublicKey          string `json:"publicKey"`
	httpclient.Base
}

type registerResponseData struct {
	Machine struct {
		ID                   string `json:"id"`
		MachineID            string `json:"machineId"`
		PublicKeyFingerprint string `json:"publicKeyFingerprint"`
	} `json:"machine"`
	Edge struct {
		Addr string `json:"addr"`
		TLS  struct {
			ServerName string `json:"serverName"`
			CAPEM      string `json:"caPem"`
		} `json:"tls"`
	} `json:"edge"`
}

func runAuthtoken(cmd *cobra.Command, args []string) error {
	workDir, err := config.ResolveWorkDir(workDirFlag)
	if err != nil {
		return err
	}
	logger.Init("", workDir)

	machineID, err := cmd.Flags().GetString("machine-id")
	if err != nil {
		return err
	}
	machineName, err := cmd.Flags().GetString("name")
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
		workDir:     workDir,
		machineID:   machineID,
		machineName: machineName,
		force:       force,
	})
	if err != nil {
		return err
	}

	logger.Info(logger.Terminal).Msg("machine registration complete")
	fmt.Println()
	pairs := registrationDetailPairs(result)
	logger.PrintDetailLines(pairs)
	fmt.Println()

	fileEvent := logger.Info(logger.File)
	for _, pair := range pairs {
		fileEvent = fileEvent.Str(pair[0], pair[1])
	}
	return nil
}

func registerMachine(ctx context.Context, opts registerOptions) (*registerResult, error) {
	workDir, err := config.ResolveWorkDir(opts.workDir)
	if err != nil {
		return nil, err
	}
	configPath := config.Path(workDir)
	if !opts.force {
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

	machineID := strings.TrimSpace(opts.machineID)

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

	reqBody := registerRequest{
		AuthToken:          authToken,
		MachineID:          machineID,
		MachineName:        machineName,
		Hostname:           hostname,
		PublicKeyAlgorithm: publicKeyAlgorithm,
		PublicKey:          publicKeyPEM,
		Base:               httpclient.Base{}.Resolve(buildmeta.SoftVersion),
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
	if strings.TrimSpace(respData.Machine.ID) == "" || strings.TrimSpace(respData.Edge.Addr) == "" {
		return nil, fmt.Errorf("register machine failed: control-plane response is missing machine registration data")
	}

	machineCfg := config.Config{
		MachineID:     respData.Machine.ID,
		EdgeAddr:      respData.Edge.Addr,
		TLSServerName: respData.Edge.TLS.ServerName,
		TLSCAPEM:      respData.Edge.TLS.CAPEM,
		PublicKeyPEM:  publicKeyPEM,
		PrivateKeyPEM: privateKeyPEM,
	}
	if err := config.Save(workDir, machineCfg); err != nil {
		return nil, err
	}

	return &registerResult{
		workDir:              workDir,
		configPath:           configPath,
		machineID:            machineCfg.MachineID,
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
		{"machine_id", result.machineID},
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
