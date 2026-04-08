# NexCtl Agent

NexCtl Agent consists of two Go programs:

- `agentd`: registration, websocket connection, heartbeat, and runtime-state reporting
- `supervisor`: process supervision and future upgrade orchestration

## Unified Server Contract

- Register: `POST /api/v1/agents/register`
- Agent websocket: `GET /api/v1/agents/ws`（`agentd` 通过请求头 `X-NexCtl-Agent-Id` / `X-NexCtl-Agent-Secret` 携带凭证）
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

## Start `agentd`

1. Edit `configs/agent.example.yaml`.
2. Run:

```powershell
go run ./cmd/agentd -config configs/agent.example.yaml
```

## Start `supervisor`

1. Edit `configs/supervisor.example.yaml`.
2. Ensure `agentd_bin` points to the built `agentd` binary.
3. Run:

```powershell
go run ./cmd/supervisor -config configs/supervisor.example.yaml
```

## Local Data Layout

- `data/config/node_key`: stable local node identity key
- `data/credentials/credential.json`: persisted long-lived server credential
- `data/logs/agentd.log`: agentd log output
- `data/logs/supervisor.log`: supervisor log output
- `data/releases`: reserved for downloaded releases
- `data/rollback`: reserved for rollback artifacts
- `data/current`: reserved for current active version pointer

## Development Defaults

- install token: `install-token-demo`
- server URL: `http://localhost:8080`
