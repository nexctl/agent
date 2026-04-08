package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Credential stores long-lived agent registration credentials.
type Credential struct {
	NodeID      int64  `json:"node_id"`
	AgentID     string `json:"agent_id"`
	AgentSecret string `json:"agent_secret"`
	WSURL       string `json:"ws_url"`
}

// Layout manages local state, config, and log directories.
type Layout struct {
	DataDir       string
	ConfigDir     string
	CredentialDir string
	LogDir        string
}

// Ensure creates the required local directories.
func (l Layout) Ensure() error {
	for _, dir := range []string{l.DataDir, l.ConfigDir, l.CredentialDir, l.LogDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

// EnsureNodeKey returns a stable local node key, creating it on first boot.
func (l Layout) EnsureNodeKey() (string, error) {
	path := filepath.Join(l.ConfigDir, "node_key")
	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		return string(raw), nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read node key: %w", err)
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate node key: %w", err)
	}
	value := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		return "", fmt.Errorf("write node key: %w", err)
	}
	return value, nil
}

// CredentialStore stores the long-lived agent credential.
type CredentialStore interface {
	Load() (*Credential, error)
	Save(credential *Credential) error
}

// FileCredentialStore stores credentials on local disk.
type FileCredentialStore struct {
	path string
}

// CredentialPath 返回 credential.json 的完整路径。
func CredentialPath(credentialDir string) string {
	return filepath.Join(credentialDir, "credential.json")
}

// RemoveCredentialFile 删除已保存的凭证文件（若不存在则忽略）。用于重装后强制重新向服务端注册。
func RemoveCredentialFile(credentialDir string) error {
	if strings.TrimSpace(credentialDir) == "" {
		return fmt.Errorf("credential_dir is empty")
	}
	path := CredentialPath(credentialDir)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// NewFileCredentialStore creates a file-backed credential store.
func NewFileCredentialStore(credentialDir string) *FileCredentialStore {
	return &FileCredentialStore{
		path: CredentialPath(credentialDir),
	}
}

// Load loads persisted credentials if present.
func (s *FileCredentialStore) Load() (*Credential, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read credential file: %w", err)
	}

	var credential Credential
	if err := json.Unmarshal(raw, &credential); err != nil {
		return nil, fmt.Errorf("unmarshal credential: %w", err)
	}
	return &credential, nil
}

// Save writes credentials to disk.
func (s *FileCredentialStore) Save(credential *Credential) error {
	raw, err := json.MarshalIndent(credential, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credential: %w", err)
	}
	if err := os.WriteFile(s.path, raw, 0o600); err != nil {
		return fmt.Errorf("write credential file: %w", err)
	}
	return nil
}
