# NexCtl Agent

单一可执行文件（对齐 [nezhahq/agent](https://github.com/nezhahq/agent) 的 `cmd/agent` 形态）：负责注册、WebSocket、心跳与运行时状态上报。

## Unified Server Contract

- Register: `POST /api/v1/agents/register`
- Agent websocket: `GET /api/v1/agents/ws`（通过请求头 `X-NexCtl-Agent-Id` / `X-NexCtl-Agent-Secret` 携带凭证）
- Persisted credential fields:
  - `node_id`
  - `agent_id`
  - `agent_secret`
  - `ws_url`

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
- 使用**非默认**配置文件路径时，会注册为独立服务名 `nexctl-agent-<配置路径 MD5 前缀>`，避免多实例冲突。
- 安装后 Linux 常见操作为：`sudo systemctl daemon-reload && sudo systemctl enable --now nexctl-agent`（具体单元名以 `install` 输出为准）。

查看子命令说明：`nexctl-agent service help`。

### 自更新（参考 [nezhahq/agent](https://github.com/nezhahq/agent)）

- 从 GitHub Releases 拉取与当前平台匹配的 zip（资源名需以 `_<goos>_<goarch>.zip` 结尾，与现有 CI 产物 `nexctl_linux_amd64.zip` 等形式一致）。
- 配置项：`disable_auto_update`、`self_update_period_minutes`（为 0 时随机约 24～48 小时检查一次，与 nezhahq 默认区间一致）、`github_repo`（默认 `nexctl/agent`）。
- 使用 `GITHUB_TOKEN` 可提升 GitHub API 限额（可选）。
- 若已成功替换二进制，进程会以退出码 1 结束，便于 systemd 等拉起新版本。

## Local Data Layout

- `data/config/node_key`: 稳定本地节点标识
- `data/credentials/credential.json`: 持久化服务端凭证
- `data/logs/agent.log`: 日志

## Development Defaults

- install token: `install-token-demo`
- server URL: `http://localhost:8080`
