package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type NL2SQLService struct {
	httpClient *http.Client

	// XiYan MCP backend (preferred when available)
	xiyanURL  string
	xiyanPath string // default: /nl2sql

	// Local LLM fallback (OpenAI-compatible, e.g., llama.cpp)
	llmBaseURL string
	llmModel   string

	// last model label used for reporting
	lastModel string
}

func NewNL2SQLService() *NL2SQLService {
	// XiYan config (preferred backend)
	xiyanURL := strings.TrimSpace(os.Getenv("XIYAN_URL"))
	xiyanPath := os.Getenv("XIYAN_SQL_PATH")
	if strings.TrimSpace(xiyanPath) == "" {
		xiyanPath = "/nl2sql"
	}

	// Local LLM (llama.cpp) fallback
	llmBaseURL := strings.TrimSpace(os.Getenv("LLM_BASE_URL"))
	if llmBaseURL == "" {
		llmBaseURL = "http://127.0.0.1:5090"
	}
	// Normalize: ensure base URL does not end with /v1 so we can append it consistently later
	llmBaseURL = strings.TrimRight(llmBaseURL, "/")
	if strings.HasSuffix(llmBaseURL, "/v1") {
		llmBaseURL = strings.TrimSuffix(llmBaseURL, "/v1")
	}
	llmModel := strings.TrimSpace(os.Getenv("LLM_MODEL"))
	// if empty, we will auto-discover from /v1/models

	// http client with timeout (longer to accommodate local CPU inference)
	client := &http.Client{
		Timeout: 180 * time.Second,
	}
	return &NL2SQLService{
		httpClient: client,
		xiyanURL:   xiyanURL,
		xiyanPath:  xiyanPath,
		llmBaseURL: llmBaseURL,
		llmModel:   llmModel,
	}
}

func (s *NL2SQLService) Model() string {
	if strings.TrimSpace(s.lastModel) != "" {
		return s.lastModel
	}
	// default label
	return "xiyan-mcp"
}

// GenerateSQL takes a natural language question and returns a safe SELECT.
// Strategy:
// 1) Try XiYan MCP if configured (XIYAN_URL). If it returns a valid SQL, use it.
// 2) Otherwise, fall back to local LLM (llama.cpp) using OpenAI-compatible API:
//   - Introspect schema via DSN or use provided schema_json
//   - Build deterministic prompt
//   - Ask model for a single fenced SQL block
//   - Sanitize to enforce SELECT-only + LIMIT
func (s *NL2SQLService) GenerateSQL(
	ctx context.Context,
	question string,
	dsnOpt *string,
	schemaJSON []byte,
	tablesWhitelist []string,
	hints []string,
	maxTables, maxCols int,
) (string, string, string, error) {
	if strings.TrimSpace(question) == "" {
		return "", "", "", errors.New("question is required")
	}

	// Resolve DSN (request overrides env)
	dsn, used := s.resolveDSN(dsnOpt)

	// Attempt XiYan first if configured
	if strings.TrimSpace(s.xiyanURL) != "" {
		if rawSQL, err := s.callXiYan(ctx, question, dsn, 100); err == nil && strings.TrimSpace(rawSQL) != "" {
			if sqlText, note, err := extractAndSanitize(rawSQL); err == nil {
				s.lastModel = "xiyan-mcp"
				return sqlText, used, note, nil
			}
		}
	}

	// Fallback: local LLM via llama.cpp (OpenAI-compatible API)
	// Build schema DDL from provided schema_json or DSN
	var (
		schema map[string][]col
		err    error
	)
	if len(schemaJSON) > 0 {
		schema, err = parseSchemaJSON(schemaJSON)
		if err != nil {
			return "", used, "", fmt.Errorf("invalid schema_json: %w", err)
		}
	} else {
		schema, err = fetchSchemaMap(ctx, dsn)
		if err != nil {
			return "", used, "", err
		}
	}

	if len(tablesWhitelist) > 0 {
		schema = filterSchemaByWhitelist(schema, tablesWhitelist)
	}
	if maxTables <= 0 {
		maxTables = 40
	}
	if maxCols <= 0 {
		maxCols = 128
	}
	ddl := renderDDLFromSchema(schema, maxTables, maxCols)
	prompt := buildPrompt(ddl, question, hints)

	raw, err := s.callLLM(ctx, prompt)
	if err != nil {
		return "", used, "", err
	}
	sqlText, note, err := extractAndSanitize(raw)
	if err != nil {
		return "", used, "", err
	}
	s.lastModel = "llama.cpp"
	return sqlText, used, note, nil
}

func (s *NL2SQLService) resolveDSN(dsnOpt *string) (string, string) {
	if dsnOpt != nil && strings.TrimSpace(*dsnOpt) != "" {
		return *dsnOpt, "request-dsn"
	}
	host := getenv("DB_HOST", "localhost")
	port := getenv("DB_PORT", "3306")
	user := getenv("DB_USER", "root")
	pass := os.Getenv("DB_PASSWORD")
	name := getenv("DB_NAME", "abdo")
	// Same options as config.ConnectDatabase
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local", user, pass, host, port, name)
	return dsn, "env"
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

type col struct {
	Name string
	Type string
}

func (s *NL2SQLService) fetchDDL(ctx context.Context, dsn string, maxTables, maxCols int) (string, error) {
	schema, err := fetchSchemaMap(ctx, dsn)
	if err != nil {
		return "", err
	}
	return renderDDLFromSchema(schema, maxTables, maxCols), nil
}

func buildPrompt(ddl, question string, hints []string) string {
	sb := &strings.Builder{}
	sb.WriteString("You are a SQL generator.\n")
	sb.WriteString("Target dialect: MySQL 8.0.\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("1) Output exactly one SELECT query; no DDL/DML; no comments.\n")
	sb.WriteString("2) Use only the provided schema; prefer deterministic constructs.\n")
	sb.WriteString("3) If the result is unbounded, add a LIMIT.\n\n")
	if len(hints) > 0 {
		sb.WriteString("Domain hints:\n")
		for _, h := range hints {
			h = strings.TrimSpace(h)
			if h == "" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(h)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Schema DDL:\n")
	sb.WriteString(ddl)
	sb.WriteString("\n\nQuestion:\n")
	sb.WriteString(question)
	sb.WriteString("\n\nReturn only the SQL in a single fenced block:\n```sql\nSELECT ...\n```")
	return sb.String()
}

func fetchSchemaMap(ctx context.Context, dsn string) (map[string][]col, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Ensure reachable
	ctxPing, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPing()
	if err := db.PingContext(ctxPing); err != nil {
		return nil, err
	}

	// Current DB
	var dbname string
	ctxDb, cancelDb := context.WithTimeout(ctx, 5*time.Second)
	defer cancelDb()
	if err := db.QueryRowContext(ctxDb, "SELECT DATABASE()").Scan(&dbname); err != nil {
		return nil, fmt.Errorf("failed to get current database: %w", err)
	}
	if strings.TrimSpace(dbname) == "" {
		return nil, errors.New("DSN must include a database (SELECT DATABASE() returned empty)")
	}

	// Columns
	q := `
SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_NAME, ORDINAL_POSITION`
	ctxQ, cancelQ := context.WithTimeout(ctx, 20*time.Second)
	defer cancelQ()
	rows, err := db.QueryContext(ctxQ, q, dbname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string][]col)
	for rows.Next() {
		var t, c, ty string
		if err := rows.Scan(&t, &c, &ty); err != nil {
			return nil, err
		}
		tables[t] = append(tables[t], col{Name: c, Type: ty})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil
}

func renderDDLFromSchema(schema map[string][]col, maxTables, maxCols int) string {
	names := make([]string, 0, len(schema))
	for t := range schema {
		names = append(names, t)
	}
	sort.Strings(names)
	if maxTables > 0 && len(names) > maxTables {
		names = names[:maxTables]
	}

	var b strings.Builder
	for i, t := range names {
		cols := schema[t]
		if maxCols > 0 && len(cols) > maxCols {
			cols = cols[:maxCols]
		}
		b.WriteString("CREATE TABLE `")
		b.WriteString(t)
		b.WriteString("` (\n")
		for j, c := range cols {
			sep := ","
			if j == len(cols)-1 {
				sep = ""
			}
			fmt.Fprintf(&b, "  `%s` %s%s\n", c.Name, c.Type, sep)
		}
		b.WriteString(");\n")
		if i < len(names)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func filterSchemaByWhitelist(schema map[string][]col, whitelist []string) map[string][]col {
	if len(whitelist) == 0 {
		return schema
	}
	out := make(map[string][]col, len(whitelist))
	for _, t := range whitelist {
		if cols, ok := schema[t]; ok {
			out[t] = cols
		}
	}
	return out
}

func parseSchemaJSON(data []byte) (map[string][]col, error) {
	type ColumnSchema struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type TableSchema struct {
		Name    string         `json:"name"`
		Columns []ColumnSchema `json:"columns"`
	}
	type JSONSchema struct {
		Tables []TableSchema `json:"tables"`
	}

	// Try typed schema: {"tables":[{"name":"orders","columns":[{"name":"id","type":"int"},...]}, ...]}
	var js JSONSchema
	if err := json.Unmarshal(data, &js); err == nil && len(js.Tables) > 0 {
		out := make(map[string][]col, len(js.Tables))
		for _, t := range js.Tables {
			if strings.TrimSpace(t.Name) == "" {
				continue
			}
			cols := make([]col, 0, len(t.Columns))
			for _, c := range t.Columns {
				if strings.TrimSpace(c.Name) == "" {
					continue
				}
				ctype := c.Type
				if strings.TrimSpace(ctype) == "" {
					ctype = "TEXT"
				}
				cols = append(cols, col{Name: c.Name, Type: ctype})
			}
			out[t.Name] = cols
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	// Try generic map: {"orders":[{"name":"id","type":"int"}, {"name":"amount","type":"decimal(10,2)"}], ...}
	var mapObj map[string][]map[string]any
	if err := json.Unmarshal(data, &mapObj); err == nil && len(mapObj) > 0 {
		out := make(map[string][]col, len(mapObj))
		for t, arr := range mapObj {
			cols := make([]col, 0, len(arr))
			for _, m := range arr {
				nv, nOk := m["name"].(string)
				tv, tOk := m["type"].(string)
				if !nOk || strings.TrimSpace(nv) == "" {
					continue
				}
				if !tOk || strings.TrimSpace(tv) == "" {
					tv = "TEXT"
				}
				cols = append(cols, col{Name: nv, Type: tv})
			}
			out[t] = cols
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	// Try very simple: {"orders":["id","amount","status"], "customers":["id","name"]}
	var mapSimple map[string][]string
	if err := json.Unmarshal(data, &mapSimple); err == nil && len(mapSimple) > 0 {
		out := make(map[string][]col, len(mapSimple))
		for t, arr := range mapSimple {
			cols := make([]col, 0, len(arr))
			for _, name := range arr {
				if strings.TrimSpace(name) == "" {
					continue
				}
				cols = append(cols, col{Name: name, Type: "TEXT"})
			}
			out[t] = cols
		}
		if len(out) > 0 {
			return out, nil
		}
	}

	return nil, errors.New("unrecognized schema_json format")
}

var (
	reSQLFenced = regexp.MustCompile("(?is)```sql\\s*(.*?)\\s*```")
	reSQLAny    = regexp.MustCompile("(?is)```\\s*(.*?)\\s*```")
)

func extractAndSanitize(text string) (string, string, error) {
	candidate := ""
	if m := reSQLFenced.FindStringSubmatch(text); len(m) >= 2 {
		candidate = m[1]
	} else if m := reSQLAny.FindStringSubmatch(text); len(m) >= 2 {
		candidate = m[1]
	} else {
		candidate = strings.TrimSpace(text)
	}
	candidate = strings.TrimSpace(candidate)
	candidate = strings.TrimSuffix(candidate, ";")
	if candidate == "" {
		return "", "", errors.New("empty model output")
	}

	upper := " " + strings.ToUpper(candidate) + " "
	if !strings.HasPrefix(strings.TrimSpace(strings.ToUpper(candidate)), "SELECT") {
		return "", "", errors.New("only SELECT statements are allowed")
	}
	bad := []string{"/*", "*/", "--", " DROP ", " DELETE ", " UPDATE ", " INSERT ", " ALTER ", " CREATE "}
	for _, tok := range bad {
		if strings.Contains(upper, tok) {
			return "", "", fmt.Errorf("unsafe token detected: %q", strings.TrimSpace(tok))
		}
	}
	note := ""
	if !strings.Contains(upper, " LIMIT ") {
		candidate += " LIMIT 100"
		note = "LIMIT 100 appended"
	}
	return candidate, note, nil
}

func (s *NL2SQLService) callXiYan(ctx context.Context, question, dsn string, limit int) (string, error) {
	if strings.TrimSpace(s.xiyanURL) == "" {
		return "", errors.New("XIYAN_URL not configured")
	}
	base := strings.TrimRight(s.xiyanURL, "/")
	path := s.xiyanPath
	if strings.TrimSpace(path) == "" {
		path = "/nl2sql"
	}
	path = strings.TrimLeft(path, "/")
	url := base + "/" + path

	// Payload shape designed to be simple and engine-agnostic.
	// XiYan MCP servers typically accept a natural language question and connection/schema context.
	// We request SQL only (no execution) for safety.
	payload := map[string]any{
		"question":   question,
		"dsn":        dsn,
		"sql_only":   true,
		"read_only":  true,
		"row_limit":  limit,
		"time_limit": 15, // seconds budget if supported by backend
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nl2sql-go/1.0 (xiyan)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("XiYan API %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// Be flexible with response shapes: {"sql": "..."}, {"generated_sql":"..."},
	// {"result":{"sql":"..."}}, {"data":{"sql":"..."}}, or a fenced block in content.
	var obj map[string]any
	if err := json.Unmarshal(bodyBytes, &obj); err == nil && len(obj) > 0 {
		// direct keys
		if v, ok := obj["sql"].(string); ok && strings.TrimSpace(v) != "" {
			return v, nil
		}
		if v, ok := obj["query"].(string); ok && strings.TrimSpace(v) != "" {
			return v, nil
		}
		if v, ok := obj["generated_sql"].(string); ok && strings.TrimSpace(v) != "" {
			return v, nil
		}
		if v, ok := obj["answer"].(string); ok && strings.TrimSpace(v) != "" {
			return v, nil
		}
		// nested objects
		if m, ok := obj["result"].(map[string]any); ok {
			if v, ok := m["sql"].(string); ok && strings.TrimSpace(v) != "" {
				return v, nil
			}
			if v, ok := m["query"].(string); ok && strings.TrimSpace(v) != "" {
				return v, nil
			}
		}
		if m, ok := obj["data"].(map[string]any); ok {
			if v, ok := m["sql"].(string); ok && strings.TrimSpace(v) != "" {
				return v, nil
			}
			if v, ok := m["query"].(string); ok && strings.TrimSpace(v) != "" {
				return v, nil
			}
		}
		// Fallback: if there's a "content" string with fenced SQL, try that
		if v, ok := obj["content"].(string); ok && strings.TrimSpace(v) != "" {
			// Let higher layer sanitizer handle fence parsing if needed
			return v, nil
		}
	}

	// Fallbacks: array or plain text
	var arr []map[string]any
	if err := json.Unmarshal(bodyBytes, &arr); err == nil && len(arr) > 0 {
		if v, ok := arr[0]["sql"].(string); ok && strings.TrimSpace(v) != "" {
			return v, nil
		}
		if v, ok := arr[0]["generated_sql"].(string); ok && strings.TrimSpace(v) != "" {
			return v, nil
		}
		if v, ok := arr[0]["content"].(string); ok && strings.TrimSpace(v) != "" {
			return v, nil
		}
	}

	// Last resort: treat body as text and let extractAndSanitize parse a ```sql block
	return string(bodyBytes), nil
}

// callLLM invokes an OpenAI-compatible /chat/completions endpoint (e.g., llama.cpp) with the prompt.
func (s *NL2SQLService) callLLM(ctx context.Context, prompt string) (string, error) {
	base := strings.TrimRight(s.llmBaseURL, "/")
	url := base + "/v1/chat/completions"

	modelID := s.llmModel
	if strings.TrimSpace(modelID) == "" {
		if m, err := s.discoverLLMModelID(ctx); err == nil && strings.TrimSpace(m) != "" {
			modelID = m
		} else {
			// Reasonable fallback for llama.cpp default we saw in /v1/models
			modelID = "./llm-models/Qwen2.5-7B-Instruct-Q4_K_M.gguf"
		}
	}

	payload := map[string]any{
		"model": modelID,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0,
		"stream":      false,
		"max_tokens":  512,
	}
	data, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nl2sql-go/1.0 (llama.cpp)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("LLM API %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	// OpenAI chat-completions shape
	var obj map[string]any
	if err := json.Unmarshal(bodyBytes, &obj); err == nil {
		if choices, ok := obj["choices"].([]any); ok && len(choices) > 0 {
			if ch, ok := choices[0].(map[string]any); ok {
				if msg, ok := ch["message"].(map[string]any); ok {
					if content, ok := msg["content"].(string); ok && strings.TrimSpace(content) != "" {
						return content, nil
					}
				}
				// some servers put "text" at the choice level
				if content, ok := ch["text"].(string); ok && strings.TrimSpace(content) != "" {
					return content, nil
				}
			}
		}
	}

	// Last resort: return raw body for upstream parsing
	return string(bodyBytes), nil
}

// discoverLLMModelID queries /v1/models to find a default model id.
func (s *NL2SQLService) discoverLLMModelID(ctx context.Context) (string, error) {
	base := strings.TrimRight(s.llmBaseURL, "/")
	url := base + "/v1/models"

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "nl2sql-go/1.0 (llama.cpp)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var obj map[string]any
	if err := json.Unmarshal(bodyBytes, &obj); err != nil {
		return "", err
	}
	if data, ok := obj["data"].([]any); ok && len(data) > 0 {
		if m, ok := data[0].(map[string]any); ok {
			if id, ok := m["id"].(string); ok && strings.TrimSpace(id) != "" {
				return id, nil
			}
		}
	}
	return "", errors.New("no models listed")
}

/* HF path removed: using XiYan MCP when available; otherwise local LLM (llama.cpp) fallback */
