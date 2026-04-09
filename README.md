# NexCtl Agent

单一可执行文件（对齐 [nezhahq/agent](https://github.com/nezhahq/agent) 的 `cmd/agent` 形态）：负责 WebSocket、心跳与运行时状态上报。接入凭据由控制台「添加节点」生成，写入 `agent.yaml` 的 `agent_id` / `agent_secret` / `node_key`（可选 `node_id`）。

## Unified Server Contract

- Agent websocket: `GET /api/v1/agents/ws`（通过请求头 `X-NexCtl-Agent-Id` / `X-NexCtl-Agent-Secret` 携带凭证）
- 配置项与历史 `credential.json`（若仍存在）中的字段：
  - `node_id`（可选）
  - `agent_id`
  - `agent_secret`
  - `ws_url`（可选；若服务端误填为 `localhost` 等回环地址，而 `server_url` 为局域网 IP，agent 会自动改用 `server_url` + `websocket_path`；也可设 `force_websocket_from_config: true`）

### WebSocket Envelope

```json
{
  "type": "runtime_state",
  "request_id": "req-123",
  "timestamp": "2026-04-08T10:30:00Z",
  "payload": {}
}
```

## 运行

1. 编辑 `configs/agent.example.yaml`。
2. 执行：

```powershell
go run ./cmd/agent -config configs/agent.example.yaml
```

构建产物默认文件名为 `nexctl-agent`（见 `.goreleaser.yml`）。发布构建会将版本写入 `internal/app.Version`（`-ldflags`），可用 `nexctl-agent -v` 查看。

### 安装为系统服务（开机自启，参考 nezhahq agent）

使用 [kardianos/service](https://github.com/kardianos/service)，支持 **Windows 服务、Linux（systemd 等）、macOS（launchd）、OpenRC 等**（与哪吒 agent 所用方案同类）。

```text
nexctl-agent service <install|uninstall|start|stop|restart|status> [-config <配置文件路径>]
```

- **install / uninstall** 通常需要 **管理员（Windows）或 root（Linux/macOS）**。
- 系统服务名固定为 **`nexctl`**（与配置文件路径无关；多实例需自行区分或使用不同机器）。
- 由 systemd / Windows SCM 拉起时会走 `kardianos/service` 的 `Run` 路径，向管理器报告已启动；**勿**在未接入 SCM 的情况下仅执行 `nexctl-agent -config` 当作服务进程，否则 Windows 易出现「未在时限内响应启动请求」(1053)。
- 安装后 Linux 常见操作为：`sudo systemctl daemon-reload && sudo systemctl enable --now nexctl`。

查看子命令说明：`nexctl-agent service help`。

### 自更新（参考 [nezhahq/agent](https://github.com/nezhahq/agent)）

- 从 GitHub Releases 拉取与当前平台匹配的 zip（资源名需以 `_<goos>_<goarch>.zip` 结尾，与现有 CI 产物 `nexctl_linux_amd64.zip` 等形式一致）。
- 配置项：`disable_auto_update`、`self_update_period_minutes`（为 0 时随机约 24～48 小时检查一次，与 nezhahq 默认区间一致）、`github_repo`（默认 `nexctl/agent`）。
- 使用 `GITHUB_TOKEN` 可提升 GitHub API 限额（可选）。
- 若已成功替换二进制，进程会以退出码 1 结束，便于 systemd 等拉起新版本。

## Local Data Layout

- `data/config/node_key`: 稳定本地节点标识
- `data/credentials/credential.json`: 持久化服务端凭证（路径由配置 `credential_dir` 决定）
- **重装覆盖凭证**：`service install` **默认会删除** `credential.json`；若仅重装服务且要保留凭证，请加 `-keep-credential`。也可单独执行 `nexctl-agent reset-credential -config <配置>`。
- `data/logs/agent.log`: 日志

## Development Defaults

- install token: `install-token-demo`
- server URL: `http://localhost:8080`
