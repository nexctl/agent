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
	RegisterPath             string `yaml:"register_path"`
	NodeName                 string `yaml:"node_name"`
	InstallToken             string `yaml:"install_token"`
	EnrollmentToken          string `yaml:"enrollment_token"`
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
	payload.Agent.RegisterPath = normalizePath(payload.Agent.RegisterPath, "/api/v1/agents/register")
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
	overrideString(&cfg.RegisterPath, "OPSPILOT_AGENT_REGISTER_PATH")
	overrideString(&cfg.NodeName, "OPSPILOT_AGENT_NODE_NAME")
	overrideString(&cfg.InstallToken, "OPSPILOT_AGENT_INSTALL_TOKEN")
	overrideString(&cfg.EnrollmentToken, "OPSPILOT_AGENT_ENROLLMENT_TOKEN")
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
	overrideInt(&cfg.SelfUpdatePeriodMinutes, "OPSPILOT_AGENT_SELF_UPDATE_PERIOD_MINUTES")
	overrideString(&cfg.GithubRepo, "OPSPILOT_AGENT_GITHUB_REPO")
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

func normalizePath(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return "/" + value
}
