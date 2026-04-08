package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nexctl/agent/internal/collector"
	"github.com/nexctl/agent/internal/config"
	"github.com/nexctl/agent/internal/store"
)

// RegisterRequest is the HTTP register request body.
type RegisterRequest struct {
	InstallToken     string `json:"install_token,omitempty"`
	EnrollmentToken  string `json:"enrollment_token,omitempty"`
	NodeKey         string `json:"node_key"`
	Name            string `json:"name"`
	Hostname        string `json:"hostname"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	Arch            string `json:"arch"`
	PrivateIP       string `json:"private_ip,omitempty"`
	PublicIP        string `json:"public_ip,omitempty"`
	AgentVersion    string `json:"agent_version"`
}

type registerEnvelope struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    *RegisterPayload `json:"data"`
}

// RegisterPayload is the server register response body.
type RegisterPayload struct {
	NodeID      int64  `json:"node_id"`
	AgentID     string `json:"agent_id"`
	AgentSecret string `json:"agent_secret"`
	WSURL       string `json:"ws_url"`
}

// Client is the HTTP client for register and future REST calls.
type Client struct {
	baseURL    string
	register   string
	httpClient *http.Client
}

// New creates a register HTTP client.
func New(cfg config.AgentConfig) *Client {
	timeout := time.Duration(cfg.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &Client{
		baseURL:  strings.TrimRight(cfg.ServerURL, "/"),
		register: cfg.RegisterPath,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Register registers the agent and returns long-lived credentials.
func (c *Client) Register(ctx context.Context, identity collector.Identity) (*store.Credential, error) {
	body := RegisterRequest{
		InstallToken:     identity.InstallToken,
		EnrollmentToken:  identity.EnrollmentToken,
		NodeKey:         identity.NodeKey,
		Name:            identity.Name,
		Hostname:        identity.Hostname,
		Platform:        identity.OS,
		PlatformVersion: identity.OSVersion,
		Arch:            identity.Arch,
		PrivateIP:       identity.PrivateIP,
		PublicIP:        identity.PublicIP,
		AgentVersion:    identity.AgentVersion,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal register request: %w", err)
	}

	endpoint, err := url.JoinPath(c.baseURL, c.register)
	if err != nil {
		return nil, fmt.Errorf("join register url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("create register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send register request: %w", err)
	}
	defer resp.Body.Close()

	var envelope registerEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode register response: %w", err)
	}
	if resp.StatusCode >= 300 || envelope.Code != 0 || envelope.Data == nil {
		return nil, fmt.Errorf("register failed: status=%d code=%d message=%s", resp.StatusCode, envelope.Code, envelope.Message)
	}

	return &store.Credential{
		NodeID:      envelope.Data.NodeID,
		AgentID:     envelope.Data.AgentID,
		AgentSecret: envelope.Data.AgentSecret,
		WSURL:       envelope.Data.WSURL,
	}, nil
}
