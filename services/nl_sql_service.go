package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"abdo/config"
)

// Upstream (Python FastAPI) request/response contracts

type NLQueryRequest struct {
	Query    string              `json:"query" binding:"required"`
	FewShots []map[string]string `json:"few_shots,omitempty"`
}

type RepairRequest struct {
	SQL         string `json:"sql" binding:"required"`
	Corrections struct {
		Tables  map[string]string `json:"tables,omitempty"`
		Columns map[string]string `json:"columns,omitempty"`
	} `json:"corrections"`
}

type FeedbackRequest struct {
	SQL   string `json:"sql" binding:"required"`
	Error string `json:"error" binding:"required"`
}

type InvalidColumn struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

type Suggestions struct {
	Tables  map[string][]string `json:"tables"`
	Columns map[string][]string `json:"columns"`
}

type NLSQLResponse struct {
	SQL            string          `json:"sql"`
	UsedTables     []string        `json:"used_tables"`
	InvalidTables  []string        `json:"invalid_tables"`
	InvalidColumns []InvalidColumn `json:"invalid_columns"`
	Suggestions    Suggestions     `json:"suggestions"`
	Valid          bool            `json:"valid"`
	SchemaSource   string          `json:"schema_source,omitempty"`
	SlicedTables   []string        `json:"sliced_tables,omitempty"`
}

// HTTP client to call the Python NL→SQL service

type NLSQLClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewNLSQLClient() *NLSQLClient {
	base := os.Getenv("NLSQL_URL")
	if base == "" {
		base = "http://127.0.0.1:7337"
	}
	timeout := 120 * time.Second
	if ts := os.Getenv("NLSQL_TIMEOUT_SECONDS"); ts != "" {
		if n, err := strconv.Atoi(ts); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Second
		}
	}
	return &NLSQLClient{
		BaseURL: strings.TrimRight(base, "/"),
		HTTP:    &http.Client{Timeout: timeout},
	}
}

func (c *NLSQLClient) postJSON(ctx context.Context, path string, reqBody any, out any) error {
	b, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("upstream %s returned status %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *NLSQLClient) NL2SQL(ctx context.Context, r NLQueryRequest) (*NLSQLResponse, error) {
	var out NLSQLResponse
	if err := c.postJSON(ctx, "/nl2sql", r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *NLSQLClient) Repair(ctx context.Context, r RepairRequest) (*NLSQLResponse, error) {
	var out NLSQLResponse
	if err := c.postJSON(ctx, "/nl2sql/repair", r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *NLSQLClient) Feedback(ctx context.Context, r FeedbackRequest) (*NLSQLResponse, error) {
	var out NLSQLResponse
	if err := c.postJSON(ctx, "/nl2sql/feedback", r, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SQL execution layer (SELECT-only, sandboxed)

type ExecResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Count   int              `json:"count"`
	SQL     string           `json:"sql"`
}

// Simple in-memory result cache for ExecuteSelect
type execCacheEntry struct {
	expiresAt time.Time
	result    *ExecResult
}

var (
	execCache   = map[string]*execCacheEntry{}
	execCacheMu sync.Mutex
)

func execCacheTTL() time.Duration {
	ttl := 300
	if v := os.Getenv("EXEC_CACHE_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttl = n
		}
	}
	return time.Duration(ttl) * time.Second
}

func execCacheMax() int {
	max := 512
	if v := os.Getenv("EXEC_CACHE_MAX_ENTRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			max = n
		}
	}
	return max
}

func normalizeSQLKey(s string) string {
	// trim, drop trailing semicolon, collapse whitespace
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ";")
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// Dialect normalizer: fixes common non-MySQL constructs emitted by LLMs.
// - INTERVAL '1 month'  => INTERVAL 1 MONTH
// - ORDER BY expr NULLS LAST  => ORDER BY expr IS NULL, expr [ASC|DESC]
// - ORDER BY expr NULLS FIRST => ORDER BY expr IS NOT NULL, expr [ASC|DESC]
var (
	reIntervalQuoted = regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)'\s+([A-Za-z]+)`)
	reNullsLast      = regexp.MustCompile(`(?i)ORDER\s+BY\s+([^\s,;]+)\s*(ASC|DESC)?\s+NULLS\s+LAST\b`)
	reNullsFirst     = regexp.MustCompile(`(?i)ORDER\s+BY\s+([^\s,;]+)\s*(ASC|DESC)?\s+NULLS\s+FIRST\b`)
)

func normalizeMySQL(s string) string {
	out := s

	// 1) INTERVAL 'n unit' -> INTERVAL n UNIT
	out = reIntervalQuoted.ReplaceAllString(out, "INTERVAL $1 $2")

	// 2) NULLS LAST
	out = reNullsLast.ReplaceAllStringFunc(out, func(m string) string {
		sub := reNullsLast.FindStringSubmatch(m)
		if len(sub) >= 3 {
			expr := sub[1]
			dir := strings.TrimSpace(sub[2])
			if dir != "" {
				dir = " " + dir
			}
			return "ORDER BY " + expr + " IS NULL, " + expr + dir
		}
		return m
	})

	// 3) NULLS FIRST
	out = reNullsFirst.ReplaceAllStringFunc(out, func(m string) string {
		sub := reNullsFirst.FindStringSubmatch(m)
		if len(sub) >= 3 {
			expr := sub[1]
			dir := strings.TrimSpace(sub[2])
			if dir != "" {
				dir = " " + dir
			}
			return "ORDER BY " + expr + " IS NOT NULL, " + expr + dir
		}
		return m
	})

	return out
}

func getFromExecCache(key string) *ExecResult {
	now := time.Now()
	execCacheMu.Lock()
	defer execCacheMu.Unlock()
	if e, ok := execCache[key]; ok {
		if now.Before(e.expiresAt) {
			return e.result
		}
		delete(execCache, key)
	}
	return nil
}

func putIntoExecCache(key string, res *ExecResult) {
	execCacheMu.Lock()
	defer execCacheMu.Unlock()
	// evict oldest if above capacity
	if len(execCache) >= execCacheMax() {
		var oldestKey string
		var oldestTime time.Time
		first := true
		for k, v := range execCache {
			if first || v.expiresAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.expiresAt
				first = false
			}
		}
		if oldestKey != "" {
			delete(execCache, oldestKey)
		}
	}
	execCache[key] = &execCacheEntry{
		expiresAt: time.Now().Add(execCacheTTL()),
		result:    res,
	}
}

func IsSelectOnly(sqlStr string) bool {
	s := strings.TrimSpace(sqlStr)
	// Block multiple statements
	if strings.Count(s, ";") > 1 {
		return false
	}
	// Allow trailing semicolon, normalize case
	sUp := strings.ToUpper(strings.TrimSpace(strings.TrimSuffix(s, ";")))
	// Allow SELECT ... or WITH ... SELECT
	if !(strings.HasPrefix(sUp, "SELECT ") || strings.HasPrefix(sUp, "WITH ")) {
		return false
	}
	// Disallow dangerous keywords
	bad := []string{
		" INSERT ", " UPDATE ", " DELETE ", " DROP ", " ALTER ", " TRUNCATE ",
		" CREATE ", " REPLACE ", " GRANT ", " REVOKE ",
	}
	for _, kw := range bad {
		if strings.Contains(" "+sUp+" ", kw) {
			return false
		}
	}
	return true
}

func ExecuteSelect(db *gorm.DB, sqlStr string) (*ExecResult, error) {
	if !IsSelectOnly(sqlStr) {
		return nil, errors.New("only SELECT queries are allowed")
	}

	// Apply MySQL dialect normalization (fix NULLS LAST, quoted INTERVAL, etc.)
	sqlStr = normalizeMySQL(sqlStr)

	// Try result cache first
	key := normalizeSQLKey(sqlStr)
	if cached := getFromExecCache(key); cached != nil {
		return cached, nil
	}

	rows, err := db.Raw(sqlStr).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	outRows := make([]map[string]any, 0, 64)
	for rows.Next() {
		raw := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		rowMap := make(map[string]any, len(cols))
		for i, col := range cols {
			switch v := raw[i].(type) {
			case []byte:
				rowMap[col] = string(v)
			default:
				rowMap[col] = v
			}
		}
		outRows = append(outRows, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	res := &ExecResult{
		Columns: cols,
		Rows:    outRows,
		Count:   len(outRows),
		SQL:     strings.TrimSpace(sqlStr),
	}
	// Put into cache
	putIntoExecCache(key, res)
	return res, nil
}

// Helper to expose *sql.DB if needed in future
func GetSQLDB() (*sql.DB, error) {
	return config.DB.DB()
}
