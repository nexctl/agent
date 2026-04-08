package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentConfig stores agentd runtime settings.
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
}

// SupervisorConfig stores supervisor runtime settings.
type SupervisorConfig struct {
	DataDir             string `yaml:"data_dir"`
	LogDir              string `yaml:"log_dir"`
	ReleaseDir          string `yaml:"release_dir"`
	RollbackDir         string `yaml:"rollback_dir"`
	CurrentDir          string `yaml:"current_dir"`
	AgentdBin           string `yaml:"agentd_bin"`
	AgentdConfig        string `yaml:"agentd_config"`
	RestartDelaySeconds int    `yaml:"restart_delay_seconds"`
	MaxRestartBurst     int    `yaml:"max_restart_burst"`
}

type agentFile struct {
	Agent AgentConfig `yaml:"agent"`
}

type supervisorFile struct {
	Supervisor SupervisorConfig `yaml:"supervisor"`
}

// LoadAgent loads agentd config from yaml and env.
func LoadAgent(path string) (AgentConfig, error) {
	var payload agentFile
	if err := load(path, &payload); err != nil {
		return AgentConfig{}, err
	}
	applyAgentEnv(&payload.Agent)
	payload.Agent.RegisterPath = normalizePath(payload.Agent.RegisterPath, "/api/v1/agents/register")
	return payload.Agent, nil
}

// LoadSupervisor loads supervisor config from yaml and env.
func LoadSupervisor(path string) (SupervisorConfig, error) {
	var payload supervisorFile
	if err := load(path, &payload); err != nil {
		return SupervisorConfig{}, err
	}
	applySupervisorEnv(&payload.Supervisor)
	return payload.Supervisor, nil
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
}

func applySupervisorEnv(cfg *SupervisorConfig) {
	overrideString(&cfg.DataDir, "OPSPILOT_SUPERVISOR_DATA_DIR")
	overrideString(&cfg.LogDir, "OPSPILOT_SUPERVISOR_LOG_DIR")
	overrideString(&cfg.ReleaseDir, "OPSPILOT_SUPERVISOR_RELEASE_DIR")
	overrideString(&cfg.RollbackDir, "OPSPILOT_SUPERVISOR_ROLLBACK_DIR")
	overrideString(&cfg.CurrentDir, "OPSPILOT_SUPERVISOR_CURRENT_DIR")
	overrideString(&cfg.AgentdBin, "OPSPILOT_SUPERVISOR_AGENTD_BIN")
	overrideString(&cfg.AgentdConfig, "OPSPILOT_SUPERVISOR_AGENTD_CONFIG")
	overrideInt(&cfg.RestartDelaySeconds, "OPSPILOT_SUPERVISOR_RESTART_DELAY_SECONDS")
	overrideInt(&cfg.MaxRestartBurst, "OPSPILOT_SUPERVISOR_MAX_RESTART_BURST")
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
