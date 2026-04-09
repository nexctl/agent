package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentConfig stores agent runtime settings.
type AgentConfig struct {
	ServerURL                string `yaml:"server_url"`
	WebSocketPath            string `yaml:"websocket_path"`
	ForceWebSocketFromConfig bool   `yaml:"force_websocket_from_config"`
	NodeName                 string `yaml:"node_name"`
	// AgentID / AgentSecret 由控制台创建节点时生成，与 node_key 一并写入配置；与 WebSocket 握手头一致。
	AgentID     string `yaml:"agent_id"`
	AgentSecret string `yaml:"agent_secret"`
	NodeKey     string `yaml:"node_key"`
	// NodeID 可选，仅用于日志；与控制台节点 ID 一致时建议填写。
	NodeID int64 `yaml:"node_id"`
	DataDir                  string `yaml:"data_dir"`
	ConfigDir                string `yaml:"config_dir"`
	CredentialDir            string `yaml:"credential_dir"`
	LogDir                   string `yaml:"log_dir"`
	RuntimeIntervalSeconds   int    `yaml:"runtime_interval_seconds"`
	HeartbeatIntervalSeconds int    `yaml:"heartbeat_interval_seconds"`
	ReconnectIntervalSeconds int    `yaml:"reconnect_interval_seconds"`
	RequestTimeoutSeconds    int    `yaml:"request_timeout_seconds"`
	AgentVersion             string `yaml:"agent_version"`
	DisableAutoUpdate        bool   `yaml:"disable_auto_update"`
	SelfUpdatePeriodMinutes  int    `yaml:"self_update_period_minutes"`
	GithubRepo               string `yaml:"github_repo"`
	TerminalShell            string `yaml:"terminal_shell"`
}

type agentFile struct {
	Agent AgentConfig `yaml:"agent"`
}

// LoadAgent loads agent config from yaml and env.
func LoadAgent(path string) (AgentConfig, error) {
	var payload agentFile
	if err := load(path, &payload); err != nil {
		return AgentConfig{}, err
	}
	applyAgentEnv(&payload.Agent)
	payload.Agent.WebSocketPath = normalizePath(payload.Agent.WebSocketPath, "/api/v1/agents/ws")
	if strings.TrimSpace(payload.Agent.GithubRepo) == "" {
		payload.Agent.GithubRepo = "nexctl/agent"
	}
	return payload.Agent, nil
}

func load(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	return nil
}

func applyAgentEnv(cfg *AgentConfig) {
	overrideString(&cfg.ServerURL, "OPSPILOT_AGENT_SERVER_URL")
	overrideString(&cfg.WebSocketPath, "OPSPILOT_AGENT_WEBSOCKET_PATH")
	overrideString(&cfg.NodeName, "OPSPILOT_AGENT_NODE_NAME")
	overrideString(&cfg.AgentID, "OPSPILOT_AGENT_AGENT_ID")
	overrideString(&cfg.AgentSecret, "OPSPILOT_AGENT_AGENT_SECRET")
	overrideString(&cfg.NodeKey, "OPSPILOT_AGENT_NODE_KEY")
	overrideInt64(&cfg.NodeID, "OPSPILOT_AGENT_NODE_ID")
	overrideString(&cfg.DataDir, "OPSPILOT_AGENT_DATA_DIR")
	overrideString(&cfg.ConfigDir, "OPSPILOT_AGENT_CONFIG_DIR")
	overrideString(&cfg.CredentialDir, "OPSPILOT_AGENT_CREDENTIAL_DIR")
	overrideString(&cfg.LogDir, "OPSPILOT_AGENT_LOG_DIR")
	overrideInt(&cfg.RuntimeIntervalSeconds, "OPSPILOT_AGENT_RUNTIME_INTERVAL_SECONDS")
	overrideInt(&cfg.HeartbeatIntervalSeconds, "OPSPILOT_AGENT_HEARTBEAT_INTERVAL_SECONDS")
	overrideInt(&cfg.ReconnectIntervalSeconds, "OPSPILOT_AGENT_RECONNECT_INTERVAL_SECONDS")
	overrideInt(&cfg.RequestTimeoutSeconds, "OPSPILOT_AGENT_REQUEST_TIMEOUT_SECONDS")
	overrideString(&cfg.AgentVersion, "OPSPILOT_AGENT_VERSION")
	if v := os.Getenv("OPSPILOT_AGENT_DISABLE_AUTO_UPDATE"); v != "" {
		cfg.DisableAutoUpdate = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("OPSPILOT_AGENT_FORCE_WEBSOCKET_FROM_CONFIG"); v != "" {
		cfg.ForceWebSocketFromConfig = strings.EqualFold(v, "true") || v == "1"
	}
	overrideInt(&cfg.SelfUpdatePeriodMinutes, "OPSPILOT_AGENT_SELF_UPDATE_PERIOD_MINUTES")
	overrideString(&cfg.GithubRepo, "OPSPILOT_AGENT_GITHUB_REPO")
	overrideString(&cfg.TerminalShell, "OPSPILOT_AGENT_TERMINAL_SHELL")
}

func overrideString(target *string, envKey string) {
	if value := os.Getenv(envKey); value != "" {
		*target = value
	}
}

func overrideInt(target *int, envKey string) {
	if value := os.Getenv(envKey); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			*target = parsed
		}
	}
}

func overrideInt64(target *int64, envKey string) {
	if value := os.Getenv(envKey); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			*target = parsed
		}
	}
}

func normalizePath(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return "/" + value
}
