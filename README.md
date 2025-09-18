# NL → SQL (Local, Free, Production‑Ready)

Fast, safe Natural Language → MySQL SELECT using a local model (llama.cpp), XiYan MCP, and this Go API as the single public endpoint. No keys. No cloud bills. Cold‑start works from your schema (DSN).

<p align="center">
  <img src="https://img.shields.io/badge/Runtime-local%20only-2ea44f?style=for-the-badge" />
  <img src="https://img.shields.io/badge/Safety-SELECT%20only-blue?style=for-the-badge" />
  <img src="https://img.shields.io/badge/MCP-XiYan-orange?style=for-the-badge" />
  <img src="https://img.shields.io/badge/Language-Go%201.21+-00ADD8?style=for-the-badge&logo=go" />
</p>

---

## TL;DR

- You ask in plain English; the API returns a single safe `SELECT` query (never executed).
- Everything runs locally: a quantized GGUF model via llama.cpp, orchestrated by XiYan MCP.
- Start three processes: local model (port 5090) → XiYan (7337) → Go API (8080).

```
curl -s -X POST http://localhost:8080/api/v1/nl2sql \
  -H 'Content-Type: application/json' \
  -d '{"question":"Orders count per status in last 30 days, descending","dsn":"root:1234@tcp(127.0.0.1:3306)/abdo?parseTime=true&charset=utf8mb4"}' | jq
```

---

## Architecture

```
(Browser/Postman)
      │
      ▼
 Go API (8080)  ── calls ──►  XiYan MCP (7337)  ── calls ──►  llama.cpp (5090, GGUF)
      │                               │                          │
      └──────────── returns one safe SELECT ◄─────────────────────┘
```

Key files:
- Controller: [`controllers.NL2SQLController.GenerateSQL()`](controllers/nl2sql_controller.go:37)
- Service: [`services.NL2SQLService.GenerateSQL()`](services/nl2sql_service.go:68)
- XiYan client: [`services.NL2SQLService.callXiYan()`](services/nl2sql_service.go:497)
- Safety filter: [`services.extractAndSanitize()`](services/nl2sql_service.go:397)
- XiYan config: [`config/xiyan.yml`](config/xiyan.yml:1)

---

## Quick Start (copy/paste)

1) Environment
- Copy example and edit DB creds:
```
cp .env.example .env
# Fill DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME
# XiYan endpoint for the Go API:
# XIYAN_URL=http://127.0.0.1:7337
# XIYAN_SQL_PATH=/nl2sql
```

2) Download model (GGUF)
- Recommended: Qwen2.5‑7B‑Instruct Q4_K_M (≈4 GB)
  - TheBloke: https://huggingface.co/TheBloke/Qwen2.5-7B-Instruct-GGUF/resolve/main/Qwen2.5-7B-Instruct-Q4_K_M.gguf?download=true
  - bartowski: https://huggingface.co/bartowski/Qwen2.5-7B-Instruct-GGUF/resolve/main/Qwen2.5-7B-Instruct-Q4_K_M.gguf?download=true
- Save to: `./llm-models/Qwen2.5-7B-Instruct-Q4_K_M.gguf`

3) Start local model server (OpenAI‑compatible)
```
python3 -m venv .venv-llama
./.venv-llama/bin/python -m pip install --upgrade pip
./.venv-llama/bin/pip install "llama-cpp-python[server]" --only-binary=:all: || \
./.venv-llama/bin/pip install "llama-cpp-python[server]"

./.venv-llama/bin/python -m llama_cpp.server \
  --model ./llm-models/Qwen2.5-7B-Instruct-Q4_K_M.gguf \
  --host 127.0.0.1 \
  --port 5090 \
  --n_ctx 4096 \
  --chat_format openai
```
- Verify: `curl -s -i http://127.0.0.1:5090/v1/models`

4) Start XiYan MCP
```
python3 -m venv .venv-xiyan
./.venv-xiyan/bin/python -m pip install --upgrade pip
./.venv-xiyan/bin/pip install xiyan-mcp-server

env YML=./config/xiyan.yml ./.venv-xiyan/bin/python -m xiyan_mcp_server
```
- Quick checks:
```
curl -s -i http://127.0.0.1:7337/sse
curl -s -i -X POST http://127.0.0.1:7337/nl2sql \
  -H "Content-Type: application/json" \
  -d '{"question":"Ping test","dsn":"root:1234@tcp(127.0.0.1:3306)/abdo?parseTime=true&charset=utf8mb4","sql_only":true,"read_only":true,"row_limit":5}'
```

5) Run the Go API
```
go run ./main.go
```
- Health: `curl -s http://127.0.0.1:8080/health`

---

## API Contract

Endpoint: `POST /api/v1/nl2sql`

Request JSON:
```
{
  "question": "Total sales by region in 2024, include region and total, top 5 descending",
  "dsn": "root:1234@tcp(127.0.0.1:3306)/abdo?parseTime=true&charset=utf8mb4"
}
```
Notes:
- `dsn` optional; if omitted, uses `.env` DB settings
- DSN must include `/dbname` so `SELECT DATABASE()` returns non‑empty

Response JSON:
```
{
  "sql": "SELECT … LIMIT 100",
  "model": "xiyan-mcp",
  "used_dsn": "env" | "request-dsn",
  "safe": true,
  "note": "LIMIT 100 appended"
}
```

---

## Test DSN vs ERP DSN (now vs later)

Today (PoC)
- We include a small Users/Orders schema to validate the NL→SQL pipeline.
- Example DSN: `root:1234@tcp(127.0.0.1:3306)/abdo?parseTime=true&charset=utf8mb4`

ERP next (staging/test)
- Provide a read‑only DSN to a staging clone of your Cetec ERP.
- Nothing changes in code: send the ERP DSN in the POST body or point `.env` to ERP and omit DSN.

Recommended MySQL user:
```
CREATE USER 'nl2sql_ro'@'%' IDENTIFIED BY 'REDACTED';
GRANT SELECT ON erp_test.* TO 'nl2sql_ro'@'%';
FLUSH PRIVILEGES;
```

---

## Safety (always on)

- Only `SELECT` statements allowed
- Blocks DDL/DML/comments (DROP/DELETE/UPDATE/INSERT/ALTER/CREATE and /* */ --)
- Auto‑appends `LIMIT 100` when unbounded

Implementation: [`services.extractAndSanitize()`](services/nl2sql_service.go:397)

---

## Troubleshooting

- XiYan prints “Loading configuration …” and nothing else
  - Ensure local model server is up: `curl http://127.0.0.1:5090/v1/models`
  - Check [`config/xiyan.yml`](config/xiyan.yml:1) → `model.url` matches your server

- llama.cpp shows “tensor … not within file bounds”
  - The `.gguf` file is corrupted or incomplete. Re‑download the GGUF (~4 GB).

- `dsn must include a database`
  - Include `/dbname` in the DSN, e.g., `/abdo`

- Ports
  - Model: 5090; XiYan: 7337; API: 8080
  - `lsof -iTCP -sTCP:LISTEN -nP | grep -E '(:5090|:7337|:8080)'`

---

## FAQ

Q: Do we need hints or example SQL?  
A: No. Cold‑start works with only your schema (via DSN). Hints are optional for ambiguous business terms.

Q: Why do we need “llama.cpp”?  
A: It’s a lightweight local server that loads your `.gguf` model and exposes a standard `/v1` HTTP API which XiYan can call.

Q: Can we switch the local runtime?  
A: Yes. Use LM Studio or Ollama; just set `model.url` in [`config/xiyan.yml`](config/xiyan.yml:1) to that server’s URL.

Q: Does the API execute SQL?  
A: Never. It returns a single safe `SELECT` string only.

---

## Code Anchors

- Controller: [`controllers.NL2SQLController.GenerateSQL()`](controllers/nl2sql_controller.go:37)
- Service: [`services.NL2SQLService.GenerateSQL()`](services/nl2sql_service.go:68)
- XiYan client: [`services.NL2SQLService.callXiYan()`](services/nl2sql_service.go:497)
- Safety: [`services.extractAndSanitize()`](services/nl2sql_service.go:397)
- XiYan config: [`config/xiyan.yml`](config/xiyan.yml:1)

---
