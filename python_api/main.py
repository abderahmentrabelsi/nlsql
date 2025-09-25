from fastapi import FastAPI, Query
from pydantic import BaseModel, Field
from typing import List, Dict, Optional, Any, Tuple, Set
from llama_cpp import Llama
import sqlparse
import json
from difflib import get_close_matches
from pathlib import Path
import os
import re
import threading
import random
import pymysql
from dotenv import load_dotenv

app = FastAPI(title="NL→SQL Service", version="0.1.0")

_llm_lock = threading.Lock()
_llm_instance: Optional[Llama] = None
_schema_cache: Optional[Dict[str, Any]] = None
_schema_lock = threading.Lock()
_project_root = Path(__file__).resolve().parents[1]
# Load environment from project root .env if available (for DB_* and toggles)
load_dotenv(str(_project_root / ".env"))

MODEL_PATH = os.getenv("LLAMA_MODEL_PATH", str(_project_root / "llm-models" / "sqlcoder-7b-2.Q4_K_M.gguf"))
SCHEMA_PATH = os.getenv("SCHEMA_PATH", str(Path(__file__).resolve().parent / "schema.json"))
PROMPT_TEMPLATE_PATH = os.getenv("PROMPT_TEMPLATE_PATH", str(Path(__file__).resolve().parent / "prompt_template.txt"))
# DB config for optional live schema reflection
DB_HOST = os.getenv("DB_HOST", "localhost")
DB_PORT = int(os.getenv("DB_PORT", "3306"))
DB_USER = os.getenv("DB_USER", "root")
DB_PASSWORD = os.getenv("DB_PASSWORD", "")
DB_NAME = os.getenv("DB_NAME", "cetecerp")
SCHEMA_DB_FALLBACK = os.getenv("SCHEMA_DB_FALLBACK", "true").lower() in ("1", "true", "yes", "y")
SCHEMA_SLICE_MAX_TABLES = int(os.getenv("SCHEMA_SLICE_MAX_TABLES", "12"))
NLSQL_CACHE_TTL_SECONDS = int(os.getenv("NLSQL_CACHE_TTL_SECONDS", "600"))
NLSQL_CACHE_MAX_ENTRIES = int(os.getenv("NLSQL_CACHE_MAX_ENTRIES", "512"))
NLSQL_N_BEST = int(os.getenv("NLSQL_N_BEST", "1"))
# in-memory NL→SQL cache: key -> (expires_at_epoch, schema_signature, response_json)
_NL_CACHE: Dict[str, Tuple[float, str, Dict[str, Any]]] = {}
_NL_CACHE_LOCK = threading.Lock()

# Warm-up state to avoid first-request cold start timeouts
_is_warm: bool = False
_warm_error: Optional[str] = None


def get_llm() -> Llama:
    global _llm_instance
    with _llm_lock:
        if _llm_instance is None:
            _llm_instance = Llama(
                model_path=MODEL_PATH,
                # faster defaults for local dev; override via env
                n_ctx=int(os.getenv("LLAMA_CTX", "2048")),
                n_threads=int(os.getenv("LLAMA_THREADS", str(os.cpu_count() or 4))),
                n_gpu_layers=int(os.getenv("LLAMA_N_GPU_LAYERS", "-1")),  # Metal offload (Apple Silicon)
                n_batch=int(os.getenv("LLAMA_N_BATCH", "128")),
                use_mmap=os.getenv("LLAMA_USE_MMAP", "true").lower() in ("1", "true", "yes", "y"),
                use_mlock=os.getenv("LLAMA_USE_MLOCK", "false").lower() in ("1", "true", "yes", "y"),
                verbose=False,
            )
        return _llm_instance


def load_schema_from_file() -> Dict[str, Any]:
    with open(SCHEMA_PATH, "r") as f:
        return json.load(f)

def _warm_up_llm() -> None:
    """Preload model and run a 1-token decode to initialize Metal/CPU kernels."""
    global _is_warm, _warm_error
    try:
        llm = get_llm()
        # 1 token warmup to compile kernels and allocate buffers
        llm(" ", max_tokens=1, temperature=0.0, top_p=1.0, stop=["\n"], echo=False)
        _is_warm = True
        _warm_error = None
    except Exception as e:
        _warm_error = str(e)
        _is_warm = False

@app.on_event("startup")
def _startup_warm():
    # Warm up in background so the server is responsive immediately
    th = threading.Thread(target=_warm_up_llm, daemon=True)
    th.start()

def load_prompt_template() -> Optional[str]:
    try:
        p = Path(PROMPT_TEMPLATE_PATH)
        if p.exists():
            return p.read_text(encoding="utf-8")
    except Exception:
        return None
    return None


def load_schema_from_db() -> Dict[str, Any]:
    # Build schema from INFORMATION_SCHEMA
    conn = pymysql.connect(
        host=DB_HOST,
        port=DB_PORT,
        user=DB_USER,
        password=DB_PASSWORD,
        database=DB_NAME,
        cursorclass=pymysql.cursors.DictCursor,
        autocommit=True,
    )
    try:
        with conn.cursor() as cur:
            # Tables and columns
            cur.execute(
                """
                SELECT TABLE_NAME, COLUMN_NAME
                FROM INFORMATION_SCHEMA.COLUMNS
                WHERE TABLE_SCHEMA = %s
                ORDER BY TABLE_NAME, ORDINAL_POSITION
                """,
                (DB_NAME,),
            )
            tables: Dict[str, List[str]] = {}
            for row in cur.fetchall():
                t = row["TABLE_NAME"]
                c = row["COLUMN_NAME"]
                tables.setdefault(t, []).append(c)

            # Relationships (FKs)
            cur.execute(
                """
                SELECT
                  TABLE_NAME AS from_table,
                  COLUMN_NAME AS from_column,
                  REFERENCED_TABLE_NAME AS to_table,
                  REFERENCED_COLUMN_NAME AS to_column
                FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
                WHERE TABLE_SCHEMA = %s
                  AND REFERENCED_TABLE_NAME IS NOT NULL
                """,
                (DB_NAME,),
            )
            relationships: List[Dict[str, str]] = []
            for row in cur.fetchall():
                relationships.append(
                    {
                        "from_table": row["from_table"],
                        "from_column": row["from_column"],
                        "to_table": row["to_table"],
                        "to_column": row["to_column"],
                    }
                )
    finally:
        conn.close()

    return {"tables": tables, "relationships": relationships}


def refresh_schema(force: bool = False) -> Dict[str, Any]:
    global _schema_cache
    with _schema_lock:
        if force or _schema_cache is None:
            _schema_cache = load_schema_from_file()
        return _schema_cache


def get_schema() -> Dict[str, Any]:
    return refresh_schema(False)


def normalize_query(nl: str) -> str:
    return " ".join(nl.lower().split())


def schema_signature(schema: Dict[str, Any]) -> str:
    try:
        return str(abs(hash(json.dumps(schema, sort_keys=True))))
    except Exception:
        # Fallback: force a unique signature if hashing fails
        return str(os.getpid())


def nl_cache_get(nl_key: str, schema_sig: str) -> Optional[Dict[str, Any]]:
    import time as _t
    now = _t.time()
    with _NL_CACHE_LOCK:
        item = _NL_CACHE.get(nl_key)
        if not item:
            return None
        exp, sig, resp = item
        if sig != schema_sig or exp <= now:
            # stale or schema changed
            _NL_CACHE.pop(nl_key, None)
            return None
        return resp


def nl_cache_set(nl_key: str, schema_sig: str, resp: Dict[str, Any]) -> None:
    import time as _t
    exp = _t.time() + max(1, NLSQL_CACHE_TTL_SECONDS)
    with _NL_CACHE_LOCK:
        # simple prune if we exceed capacity
        if len(_NL_CACHE) >= max(1, NLSQL_CACHE_MAX_ENTRIES):
            # drop the earliest expiring entry
            oldest_k = None
            oldest_exp = 0.0
            for k, (e, _, _) in _NL_CACHE.items():
                if oldest_k is None or e < oldest_exp:
                    oldest_k = k
                    oldest_exp = e
            if oldest_k:
                _NL_CACHE.pop(oldest_k, None)
        _NL_CACHE[nl_key] = (exp, schema_sig, resp)


def nl_cache_clear() -> None:
    with _NL_CACHE_LOCK:
        _NL_CACHE.clear()


def tokenize_nl(nl: str) -> Set[str]:
    text = nl.lower()
    toks = set(re.findall(r"[A-Za-z_][A-Za-z0-9_]*", text))
    # add simple singulars
    singulars = set()
    for t in list(toks):
        if len(t) > 3 and t.endswith("s"):
            singulars.add(t[:-1])
    return toks.union(singulars)


def slice_schema_for_query(nl: str, schema: Dict[str, Any]) -> Tuple[Dict[str, Any], Set[str]]:
    tokens = tokenize_nl(nl)
    tables_def: Dict[str, List[str]] = schema.get("tables", {})
    rels: List[Dict[str, str]] = schema.get("relationships", [])
    scores: Dict[str, int] = {}
    for t, cols in tables_def.items():
        t_low = t.lower()
        score = 0
        if t_low in tokens:
            score += 10
        for tok in tokens:
            if tok and tok in t_low:
                score += 2
        for c in cols:
            c_low = c.lower()
            if c_low in tokens:
                score += 5
            else:
                for tok in tokens:
                    if tok and tok in c_low:
                        score += 1
        if score > 0:
            scores[t] = score
    # choose top tables by score
    selected: List[str] = []
    for t, _ in sorted(scores.items(), key=lambda kv: kv[1], reverse=True):
        if len(selected) >= SCHEMA_SLICE_MAX_TABLES:
            break
        selected.append(t)
    # if nothing matched, pick a small default slice
    if not selected:
        defaults = [t for t in ("ordhead", "ordline", "customers") if t in tables_def]
        selected = defaults or list(tables_def.keys())[: min(SCHEMA_SLICE_MAX_TABLES, len(tables_def))]
    # expand with neighbors via relationships (to enable joins)
    sel_set: Set[str] = set(selected)
    for r in rels:
        if len(sel_set) >= SCHEMA_SLICE_MAX_TABLES:
            break
        if r.get("from_table") in sel_set and r.get("to_table") not in sel_set:
            sel_set.add(r.get("to_table"))
        if r.get("to_table") in sel_set and r.get("from_table") not in sel_set:
            sel_set.add(r.get("from_table"))
    # build sliced schema
    sliced_tables = {t: tables_def[t] for t in sel_set if t in tables_def}
    sliced_rels = [r for r in rels if r.get("from_table") in sel_set and r.get("to_table") in sel_set]
    sliced: Dict[str, Any] = {"tables": sliced_tables, "relationships": sliced_rels}
    # include services if present (unchanged; optional)
    if "services" in schema:
        sliced["services"] = schema["services"]
    return sliced, sel_set


class NLQuery(BaseModel):
    query: str = Field(..., description="Natural language question")
    few_shots: Optional[List[Dict[str, str]]] = None


class RepairRequest(BaseModel):
    sql: str
    corrections: Dict[str, Dict[str, str]] = Field(default_factory=dict)


class FeedbackRequest(BaseModel):
    sql: str
    error: str


def build_few_shots(schema: Dict[str, Any]) -> List[Dict[str, str]]:
    examples: List[Dict[str, str]] = []
    # 1) Sales by customer (ordhead ↔ customers)
    examples.append({
        "nl": "Top 10 customers by invoice total in the last 30 days",
        "sql": "SELECT c.id, c.custnum, c.name, SUM(o.invoice_total) AS total_invoice "
               "FROM ordhead o "
               "LEFT JOIN customers c ON c.id = o.customer_id "
               "WHERE o.oorderdate >= DATE_SUB(CURRENT_DATE, INTERVAL 30 DAY) "
               "GROUP BY c.id, c.custnum, c.name "
               "ORDER BY total_invoice DESC "
               "LIMIT 10;"
    })
    # 2) Open orders by status
    examples.append({
        "nl": "Count of open orders by status code",
        "sql": "SELECT o.orderstatus AS status, COUNT(*) AS cnt "
               "FROM ordhead o "
               "WHERE o.deleted_flag = 0 "
               "GROUP BY o.orderstatus "
               "ORDER BY cnt DESC;"
    })
    # 3) Order lines for a specific order number
    examples.append({
        "nl": "List the lines for order number 100234 with part and quantities",
        "sql": "SELECT l.id, l.ordernum, l.lineitem, l.prcpart, l.oorderqty, l.shipqty, l.status "
               "FROM ordline l "
               "WHERE l.ordernum = '100234' "
               "ORDER BY l.lineitem;"
    })
    # 4) PO spend by vendor (pos ↔ vendors)
    examples.append({
        "nl": "Total PO spend by vendor over the last 90 days",
        "sql": "SELECT v.id, v.vennum, v.name, SUM(pl.unit_cost * COALESCE(pl.orderqty, 0)) AS total_spend "
               "FROM pos p "
               "JOIN po_lines pl ON pl.po_id = p.id "
               "LEFT JOIN vendors v ON v.id = p.vendor_id "
               "WHERE p.enterdate >= DATE_SUB(CURRENT_DATE, INTERVAL 90 DAY) "
               "GROUP BY v.id, v.vennum, v.name "
               "ORDER BY total_spend DESC;"
    })
    return examples


def build_prompt(nl: str, schema: Dict[str, Any], few_shots: Optional[List[Dict[str, str]]] = None) -> str:
    # compact JSON to reduce prompt tokens
    schema_text = json.dumps(schema, separators=(",", ":"), ensure_ascii=False)
    if not few_shots:
        few_shots = build_few_shots(schema)
    shots = ""
    for ex in few_shots:
        shots += f"User: {ex['nl']}\nAssistant:\n{ex['sql']}\n\n"

    # optional synonyms block from schema.json -> { "synonyms": { "cost": "orders.amount", ... } }
    synonyms_text = ""
    syn = schema.get("synonyms")
    if isinstance(syn, dict) and syn:
        lines = []
        for k, v in syn.items():
            if isinstance(v, list):
                vv = ", ".join(map(str, v))
            else:
                vv = str(v)
            lines.append(f"{k}: {vv}")
        syn_limit = int(os.getenv("PROMPT_SYNONYMS_MAX", "12"))
        if syn_limit > 0:
            lines = lines[:syn_limit]
        synonyms_text = "\n".join(lines)

    # If a prompt template file exists, use it with simple marker replacement
    tpl = load_prompt_template()
    if tpl:
        rules_default = "- Use only tables and columns present in the schema.\n- Prefer explicit JOINs using relationships and FK directions.\n- Output a single MySQL SELECT ending with a semicolon.\n- No explanations, no markdown."
        prompt = (
            tpl.replace("{{SCHEMA}}", schema_text)
               .replace("{{NL}}", nl)
               .replace("{{FEWSHOTS}}", shots)
               .replace("{{SYNONYMS}}", synonyms_text)
               .replace("{{RULES}}", rules_default)
        )
        return prompt

    # Fallback built-in template
    prompt = f"""You are a senior SQL engineer generating MySQL SQL. Use ONLY the provided schema and relationships. Output ONLY a single SQL query without explanations or markdown.

Schema JSON:
{schema_text}

Rules:
- Use only tables and columns that exist in the schema.
- Prefer explicit JOINs using relationships.
- Return SELECT queries only, safe and deterministic.
- Do not wrap SQL in code fences.
- End with a semicolon.

Synonyms (optional):
{synonyms_text}

{shots}User: {nl}
Assistant:
"""
    return prompt


def run_llm(prompt: str) -> str:
    llm = get_llm()
    out = llm(
        prompt=prompt,
        temperature=0.0,
        max_tokens=int(os.getenv("LLAMA_MAX_TOKENS", "128")),
        top_p=1.0,
        stop=["User:", "Assistant:", "\n\n"],
        echo=False,
    )
    text = out["choices"][0]["text"]
    sql = text.strip()
    # Extract SQL if extra text
    fence = re.search(r"```(?:sql)?(.*?)```", sql, re.S | re.I)
    if fence:
        sql = fence.group(1).strip()
    # Keep from first SELECT
    m = re.search(r"(?is)select\b.*", sql)
    if m:
        sql = m.group(0).strip()
    # Ensure single trailing semicolon
    if not sql.endswith(";"):
        sql += ";"
    return sql


def _rotate_shots(shots: List[Dict[str, str]], k: int) -> List[Dict[str, str]]:
    if not shots:
        return []
    k = k % len(shots)
    return shots[k:] + shots[:k]


def _pick_best(candidates: List[Tuple[str, Dict[str, Any]]]) -> Tuple[str, Dict[str, Any]]:
    # Sort: prefer valid, then fewer invalid refs, then shorter SQL
    def key(item: Tuple[str, Dict[str, Any]]):
        sql, analysis = item
        invalid_tables = len(analysis.get("invalid_tables", []))
        invalid_cols = len(analysis.get("invalid_columns", []))
        return (not bool(analysis.get("valid")), invalid_tables + invalid_cols, len(sql))
    candidates_sorted = sorted(candidates, key=key)
    return candidates_sorted[0]


def parse_tables_and_aliases(sql: str) -> Tuple[Set[str], Dict[str, str]]:
    tables: Set[str] = set()
    alias_to_table: Dict[str, str] = {}
    pattern = re.compile(r"\b(?:FROM|JOIN)\s+[`\"]?(\w+)[`\"]?(?:\s+(?:AS\s+)?(\w+))?", re.I)
    for m in pattern.finditer(sql):
        table = m.group(1)
        alias = m.group(2)
        tables.add(table)
        if alias:
            alias_to_table[alias] = table
    return tables, alias_to_table


def parse_columns(sql: str, alias_to_table: Dict[str, str]) -> List[Tuple[Optional[str], str]]:
    # Collect references and avoid duplicates
    refs: Set[Tuple[Optional[str], str]] = set()

    # 1) Capture qualified refs anywhere (t.col), including inside functions
    for q in re.finditer(r"\b[`\"]?(\w+)[`\"]?\.[`\"]?(\w+)[`\"]?\b", sql):
        qual, col = q.group(1), q.group(2)
        refs.add((qual, col))

    # 2) Capture plausible unqualified refs from the SELECT list only, ignoring functions/keywords
    m = re.search(r"(?is)\bselect\b(.*?)\bfrom\b", sql)
    if m:
        select_part = m.group(1)

        # split by top-level commas (ignore commas inside parentheses)
        items: List[str] = []
        buf: List[str] = []
        depth = 0
        for ch in select_part:
            if ch == "(":
                depth += 1
            elif ch == ")":
                depth = max(0, depth - 1)
            if ch == "," and depth == 0:
                items.append("".join(buf).strip())
                buf = []
            else:
                buf.append(ch)
        if buf:
            items.append("".join(buf).strip())

        FUNCTIONS = {
            "SUM","COUNT","AVG","MIN","MAX","DATE_SUB","NOW","CURRENT_DATE","COALESCE",
            "ROUND","ABS","UPPER","LOWER","LENGTH","YEAR","MONTH","DAY","DATE","CAST",
            "IF","IFNULL"
        }
        KEYWORDS = {"NULL","TRUE","FALSE","CASE","WHEN","THEN","END","DISTINCT","ALL","AS"}

        for it in items:
            # remove alias e.g., "expr AS alias"
            it = re.split(r"(?i)\bas\b", it)[0].strip()
            if it == "*":
                continue
            # already covered qualified refs (t.col)
            if "." in it and re.search(r"\w\.\w", it):
                continue
            # strip quotes/backticks
            token = re.sub(r"[`\"']", "", it).strip()
            # skip function names (e.g., SUM(...))
            head = token.split("(", 1)[0].strip()
            if head.upper() in FUNCTIONS or head.upper() in KEYWORDS:
                continue
            # add plain identifier (likely an unqualified column)
            if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*", token):
                refs.add((None, token))

    # Normalize qualifiers through alias map
    norm: List[Tuple[Optional[str], str]] = []
    for qual, col in refs:
        if qual and qual in alias_to_table:
            norm.append((alias_to_table[qual], col))
        elif qual:
            norm.append((qual, col))
        else:
            norm.append((None, col))
    return norm


def validate_sql(sql: str, schema: Dict[str, Any]) -> Dict[str, Any]:
    tables_def: Dict[str, List[str]] = schema.get("tables", {})
    all_tables = set(tables_def.keys())
    used_tables, alias_map = parse_tables_and_aliases(sql)
    invalid_tables = sorted([t for t in used_tables if t not in all_tables])

    column_refs = parse_columns(sql, alias_map)
    invalid_columns: List[Dict[str, str]] = []
    for qual, col in column_refs:
        if qual:
            base = qual
            if base not in tables_def:
                invalid_columns.append({"table": base, "column": col})
            else:
                if col not in tables_def[base]:
                    invalid_columns.append({"table": base, "column": col})
        else:
            # unqualified: accept if unique across used tables
            candidates = [t for t in used_tables if t in tables_def and col in tables_def[t]]
            if len(candidates) == 0:
                invalid_columns.append({"table": "", "column": col})

    # Suggestions
    suggestions = {"tables": {}, "columns": {}}
    for t in invalid_tables:
        suggestions["tables"][t] = get_close_matches(t, list(all_tables), n=3, cutoff=0.6)
    # Column suggestions per table
    all_columns = {}
    for t, cols in tables_def.items():
        for c in cols:
            all_columns[f"{t}.{c}"] = (t, c)
    for pair in invalid_columns:
        t = pair["table"]
        c = pair["column"]
        if t and t in tables_def:
            suggestions["columns"][f"{t}.{c}"] = get_close_matches(c, tables_def[t], n=3, cutoff=0.6)
        else:
            suggestions["columns"][c] = [cn.split(".", 1)[1] for cn in get_close_matches(c, list(all_columns.keys()), n=3, cutoff=0.6)]

    valid = len(invalid_tables) == 0 and len(invalid_columns) == 0
    return {
        "used_tables": sorted(list(used_tables)),
        "invalid_tables": invalid_tables,
        "invalid_columns": invalid_columns,
        "suggestions": suggestions,
        "valid": valid
    }


def apply_corrections(sql: str, corrections: Dict[str, Dict[str, str]]) -> str:
    # Replace tables
    for wrong, right in corrections.get("tables", {}).items():
        sql = re.sub(rf"\b{re.escape(wrong)}\b", right, sql)
    # Replace columns (qualified or not)
    for wrong, right in corrections.get("columns", {}).items():
        # if wrong is qualified, replace as is; also try unqualified column name
        parts = wrong.split(".")
        if len(parts) == 2:
            _, col = parts
        else:
            col = wrong
        sql = re.sub(rf"\b{re.escape(wrong)}\b", right, sql)
        sql = re.sub(rf"(?<!\.)\b{re.escape(col)}\b", right.split(".")[-1], sql)
    return sql


@app.get("/health")
def health():
    return {
        "status": "ok",
        "model_path": MODEL_PATH,
        "schema_path": SCHEMA_PATH,
        "prompt_template_path": PROMPT_TEMPLATE_PATH,
        "prompt_template_exists": Path(PROMPT_TEMPLATE_PATH).exists(),
        "db_fallback": SCHEMA_DB_FALLBACK,
        "db_name": DB_NAME,
        "n_best": NLSQL_N_BEST,
        "warm": _is_warm,
        "warm_error": _warm_error,
    }


@app.get("/schema")
def schema_endpoint(
    reload: bool = Query(False, description="Force reload of schema from DB/file"),
    q: Optional[str] = Query(None, description="Natural language query to slice schema for"),
    do_slice: bool = Query(False, alias="slice", description="Return sliced schema subset for the provided query"),
):
    base = refresh_schema(force=reload)
    if reload:
        nl_cache_clear()
    if do_slice and q:
        sliced, selected = slice_schema_for_query(q, base)
        return {"schema": sliced, "selected_tables": sorted(list(selected))}
    return base


@app.post("/nl2sql")
def nl2sql_endpoint(payload: NLQuery):
    # If model is still warming, fail fast to avoid long frontend timeouts
    if not _is_warm:
        return {"error": "warming_up", "detail": "Model is warming up, retry shortly."}, 503

    # 1) JSON-first with cache
    base_schema = load_schema_from_file()
    sig = schema_signature(base_schema)
    nl_key = normalize_query(payload.query)
    cached = nl_cache_get(nl_key, sig)
    if cached is not None:
        return cached

    # 2) Build with sliced JSON schema
    sliced1, selected1 = slice_schema_for_query(payload.query, base_schema)

    # n-best with rotated few-shots to provide slight diversity while staying deterministic
    k = max(1, NLSQL_N_BEST)
    base_shots = build_few_shots(base_schema)[:max(0, int(os.getenv("FEW_SHOT_MAX", "2")))]
    candidates: List[Tuple[str, Dict[str, Any]]] = []
    for i in range(k):
        shots_i = _rotate_shots(base_shots, i)
        prompt_i = build_prompt(payload.query, sliced1, shots_i)
        try:
            sql_i = run_llm(prompt_i)
        except ValueError as e:
            if "context window" in str(e).lower():
                tiny_tables = dict(list(sliced1.get("tables", {}).items())[:6])
                tiny_schema = {"tables": tiny_tables, "relationships": []}
                try:
                    # reduce few-shots aggressively
                    sql_i = run_llm(build_prompt(payload.query, tiny_schema, shots_i[:1]))
                except Exception:
                    return JSONResponse({"error": "prompt_too_large", "detail": "Prompt exceeds context; try a narrower question"}, status_code=413)
            else:
                return JSONResponse({"error": "llm_error", "detail": str(e)}, status_code=500)
        analysis_i = validate_sql(sql_i, base_schema)
        candidates.append((sql_i, analysis_i))
    sql_best, analysis_best = _pick_best(candidates)

    resp: Dict[str, Any] = {
        "sql": sql_best,
        **analysis_best,
        "schema_source": "json",
        "sliced_tables": sorted(list(selected1)),
    }

    # 3) Optional DB fallback only if JSON is missing objects
    if not analysis_best.get("valid") and SCHEMA_DB_FALLBACK and DB_NAME:
        db_schema = load_schema_from_db()
        needs_retry = False
        for t in analysis_best.get("invalid_tables", []):
            if t in db_schema.get("tables", {}):
                needs_retry = True
                break
        if not needs_retry:
            for pair in analysis_best.get("invalid_columns", []):
                t = pair.get("table")
                c = pair.get("column")
                if t and t in db_schema.get("tables", {}) and c in db_schema["tables"][t]:
                    needs_retry = True
                    break
        if needs_retry:
            sliced2, selected2 = slice_schema_for_query(payload.query, db_schema)
            base_shots_db = build_few_shots(db_schema)
            candidates_db: List[Tuple[str, Dict[str, Any]]] = []
            for i in range(k):
                shots_i = _rotate_shots(base_shots_db, i)
                sql_i = run_llm(build_prompt(payload.query, sliced2, shots_i))
                analysis_i = validate_sql(sql_i, db_schema)
                candidates_db.append((sql_i, analysis_i))
            sql2, analysis2 = _pick_best(candidates_db)
            resp = {
                "sql": sql2,
                **analysis2,
                "schema_source": "db_fallback",
                "sliced_tables": sorted(list(selected2)),
            }

    # 4) Cache final response under JSON schema signature
    nl_cache_set(nl_key, sig, resp)
    return resp


@app.post("/nl2sql/repair")
def repair_endpoint(req: RepairRequest):
    schema = get_schema()
    new_sql = apply_corrections(req.sql, req.corrections or {})
    analysis = validate_sql(new_sql, schema)
    return {"sql": new_sql, **analysis}


@app.post("/nl2sql/feedback")
def feedback_endpoint(req: FeedbackRequest):
    schema = get_schema()
    prompt = f"""{build_prompt("Fix the following SQL based on error.", schema)}
Previous SQL:
{req.sql}

The previous SQL returned this error: {req.error}
Fix it using only available schema and relationships. Output only a valid SELECT SQL;"""
    sql = run_llm(prompt)
    analysis = validate_sql(sql, schema)
    return {"sql": sql, **analysis}