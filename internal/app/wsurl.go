package app

import (
	"net/url"
	"strings"

	"github.com/nexctl/agent/internal/config"
	"go.uber.org/zap"
)

// resolveWebSocketDialURL 决定传给 WebSocket 客户端的地址。
// 返回空字符串表示由 wsclient 根据 server_url + websocket_path 推导（与 credential 中 ws_url 无关）。
func resolveWebSocketDialURL(logger *zap.Logger, cfg config.AgentConfig, credentialWSURL string) string {
	if cfg.ForceWebSocketFromConfig {
		logger.Info("websocket: 使用 server_url + websocket_path（force_websocket_from_config）")
		return ""
	}
	if shouldPreferServerURLForWS(cfg.ServerURL, credentialWSURL) {
		logger.Warn("websocket: 注册返回的 ws_url 指向本机回环，但 server_url 不是回环，已改为使用 server_url + websocket_path",
			zap.String("ignored_ws_url", strings.TrimSpace(credentialWSURL)),
			zap.String("server_url", cfg.ServerURL))
		return ""
	}
	return credentialWSURL
}

// shouldPreferServerURLForWS：服务端误把 ws_url 写成 localhost 时，本地仍用局域网 IP 访问（常见配置错误）。
func shouldPreferServerURLForWS(serverURL, credWS string) bool {
	su, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil || su.Host == "" {
		return false
	}
	cu, err := url.Parse(strings.TrimSpace(credWS))
	if err != nil || cu.Host == "" {
		return false
	}
	credHost := strings.ToLower(cu.Hostname())
	srvHost := strings.ToLower(su.Hostname())
	if credHost == "" || srvHost == "" {
		return false
	}
	return isLoopbackHost(credHost) && !isLoopbackHost(srvHost)
}

func isLoopbackHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "ip6-localhost", "ip6-loopback":
		return true
	default:
		return false
	}
}
