# NL→SQL for ERP — Go + FastAPI + llama.cpp (Qwen GGUF)

Production-grade natural language to SQL pipeline built around a local Qwen 2.5 7B Instruct (GGUF) model via llama.cpp, a Python FastAPI microservice for generation/validation, and a Go API for safe SQL execution and integration.

Key goals

- High precision and accuracy on ERP-style queries
- Deterministic generation (temperature 0) with schema grounding
- Hallucination detection with interactive repair suggestions
- Safe, SELECT-only execution with transparent SQL

Architecture

```
 ┌──────────────────────────┐        ┌──────────────────────────────────────┐
 │        Frontend         │  NL    │                Go API                │
 │  (forms + dropdown UX)  ├────────▶  /api/v1/nl2sql/* routes             │
 └──────────────────────────┘        │  routes/nl_sql_routes.go             │
                                      │  controllers/nl_sql_controller.go    │
                                      │  services/nl_sql_service.go          │
                                      └──────────────┬───────────────┬──────┘
                                                     │               │
                                          Generate/Repair/Feedback   │ Execute (SELECT-only)
                                                     │               │
                                                     ▼               ▼
                                      ┌──────────────────────────────────────┐
                                      │          Python FastAPI NLSQL        │
                                      │  python_api/main.py                  │
                                      │  - Schema grounding from             │
                                      │    python_api/schema.json            │
                                      │  - llama.cpp via Qwen GGUF           │
                                      │  - SQL validation + suggestions      │
                                      └───────────────────┬──────────────────┘
                                                          │
                                                          ▼
                                          ┌────────────────────────────────┐
                                          │        MySQL (via GORM)        │
                                          │  config/database.go            │
                                          │  models/*.go                   │
                                          └────────────────────────────────┘
```

Repository layout

- [main.go](main.go): bootstraps the Go server and routes
- [config/database.go](config/database.go): DB connection + migrations
- [models/user.go](models/user.go), [models/order.go](models/order.go): sample ERP tables
- [services/nl_sql_service.go](services/nl_sql_service.go): HTTP client to Python and safe SQL execution
- [controllers/nl_sql_controller.go](controllers/nl_sql_controller.go): NL→SQL/repair/feedback/execute endpoints
- [routes/nl_sql_routes.go](routes/nl_sql_routes.go), [routes/routes.go](routes/routes.go): route wiring
- [python_api/main.py](python_api/main.py): llama.cpp-backed generator + validator
- [python_api/schema.json](python_api/schema.json): ERP schema + relationships
- [llm-models/Qwen2.5-7B-Instruct-Q4_K_M.gguf](llm-models/Qwen2.5-7B-Instruct-Q4_K_M.gguf): local model

Quick start

Prerequisites

- Python 3.10+ and Go 1.21+
- MySQL running and reachable (for execute)
- jq: brew install jq

1) Start Python NL→SQL service

```
cd ./python_api
python3 -m venv .venv && source .venv/bin/activate
pip install --upgrade pip
pip install "llama-cpp-python==0.2.86" fastapi uvicorn pydantic sqlparse
export LLAMA_MODEL_PATH="$(pwd)/../llm-models/Qwen2.5-7B-Instruct-Q4_K_M.gguf"
# Optional tuning
export LLAMA_THREADS=8
export LLAMA_CTX=4096
uvicorn main:app --host 127.0.0.1 --port 7337
```

Warm the model once (first call can take 1–3 minutes):

```
curl -sS -X POST http://127.0.0.1:7337/nl2sql \
  -H 'Content-Type: application/json' \
  -d '{"query":"ping"}' | jq .
```

2) Start Go API

```
cd ./    # project root
test -f .env || cp .env.example .env
export NLSQL_URL=http://127.0.0.1:7337
export NLSQL_TIMEOUT_SECONDS=600   # allow long first request
go run .
```

Health checks

```
curl -s http://127.0.0.1:7337/health | jq .
curl -s http://localhost:8080/health | jq .
```

End-to-end usage

Generate SQL

```
curl -sS -X POST http://localhost:8080/api/v1/nl2sql \
  -H 'Content-Type: application/json' \
  -d '{"query":"total order amount per user"}' | jq .
```

Repair with suggestions (simulate hallucinated tables)

```
curl -sS -X POST http://localhost:8080/api/v1/nl2sql/repair \
  -H 'Content-Type: application/json' \
  -d '{"sql":"SELECT oh.order_date, ol.item_name FROM orderhead oh JOIN orderline ol ON oh.id = ol.order_id;","corrections":{"tables":{"orderhead":"orders","orderline":"orders"}}}' | jq .
```

Execute generated SQL (SELECT-only; requires DB)

```
SQL=$(curl -sS -X POST http://localhost:8080/api/v1/nl2sql \
  -H 'Content-Type: application/json' \
  -d '{"query":"total order amount per user"}' | jq -r '.sql')
jq -nc --arg sql "$SQL" '{sql:$sql}' | curl -sS -X POST \
  http://localhost:8080/api/v1/nl2sql/execute \
  -H 'Content-Type: application/json' \
  --data-binary @- | jq .
```

Error feedback loop (let the model fix real DB errors)

```
curl -sS -X POST http://localhost:8080/api/v1/nl2sql/feedback \
  -H 'Content-Type: application/json' \
  -d '{"sql":"SELECT u.name, SUM(o.ammount) FROM users u JOIN orders o ON o.user_id=u.id GROUP BY u.name;","error":"Unknown column ammount in field list"}' | jq .
```

How it works

1) Schema grounding
   - Full ERP schema and relationships live in [python_api/schema.json](python_api/schema.json).
   - The Python service injects this JSON, plus few-shot examples, into the prompt to constrain generation.

2) Deterministic generation
   - llama.cpp with Qwen 2.5 7B Instruct GGUF, temperature 0, bounded max tokens.
   - Prompt crafted to emit a single MySQL SELECT statement ending with a semicolon.

3) Validation and suggestions
   - SQL is parsed to extract used tables and columns.
   - Any table/column not in the schema is flagged as a hallucination.
   - Closest matches suggested via difflib to repair interactively.

4) Repair
   - Frontend can post corrections (tables/columns); backend applies them and re-validates.

5) Error feedback loop
   - If execution fails (e.g., “Unknown column”), the error text is fed back into a corrective prompt for regeneration.

6) Safe execution
   - Go service enforces single-statement, SELECT-only, and blocks DDL/DML keywords before execution.
   - Execution happens via GORM’s Raw on the configured MySQL connection.

API surface

Go API (prefix /api/v1)

- POST /nl2sql → generate SQL + validation
- POST /nl2sql/repair → apply user corrections and revalidate
- POST /nl2sql/feedback → regenerate using DB error feedback
- POST /nl2sql/execute → execute validated SELECT and return rows

Python FastAPI

- POST /nl2sql → generate and validate SQL
- POST /nl2sql/repair → apply corrections and validate
- POST /nl2sql/feedback → fix SQL using error text
- GET /health → basic status

Safety, constraints, and transparency

- SELECT-only enforcement and multi-statement blocking in the Go layer
- SQL always returned to users for transparency
- No schema writes; migrations only at startup via GORM

Performance and tuning

- First request warms the model and can take 1–3 minutes; later calls are fast
- Environment variables:
  - LLAMA_THREADS, LLAMA_CTX, LLAMA_MAX_TOKENS
  - NLSQL_TIMEOUT_SECONDS (Go client timeout)
- Keep few-shot examples concise to reduce latency

Troubleshooting

- “context deadline exceeded”: increase NLSQL_TIMEOUT_SECONDS and warm Python with a direct call
- “Unknown column/table”: use /nl2sql/repair suggestions or feed the exact DB error to /nl2sql/feedback
- Very slow generations: reduce LLAMA_MAX_TOKENS or LLAMA_CTX, or try a smaller quantization

Roadmap (optional)

- Schema slicing (only the relevant subset of tables for the query)
- Query/result caching
- Column-level dropdown suggestions in UI
- Richer few-shots covering aggregates/filters/joins

Credits

- Local model: [llm-models/Qwen2.5-7B-Instruct-Q4_K_M.gguf](llm-models/Qwen2.5-7B-Instruct-Q4_K_M.gguf)
- Generation/validation service: [python_api/main.py](python_api/main.py)
- Go integration and execution: [services/nl_sql_service.go](services/nl_sql_service.go), [controllers/nl_sql_controller.go](controllers/nl_sql_controller.go), [routes/nl_sql_routes.go](routes/nl_sql_routes.go)

## Author

- Abderahmen Trabelsi — GitHub: [abderahmentrabelsi](https://github.com/abderahmentrabelsi)
