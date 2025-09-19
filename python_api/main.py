from fastapi import FastAPI
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

app = FastAPI(title="NL→SQL Service", version="0.1.0")

_llm_lock = threading.Lock()
_llm_instance: Optional[Llama] = None
_schema_cache: Optional[Dict[str, Any]] = None
_project_root = Path(__file__).resolve().parents[1]

MODEL_PATH = os.getenv("LLAMA_MODEL_PATH", str(_project_root / "llm-models" / "Qwen2.5-7B-Instruct-Q4_K_M.gguf"))
SCHEMA_PATH = os.getenv("SCHEMA_PATH", str(Path(__file__).resolve().parent / "schema.json"))


def get_llm() -> Llama:
    global _llm_instance
    with _llm_lock:
        if _llm_instance is None:
            _llm_instance = Llama(
                model_path=MODEL_PATH,
                n_ctx=int(os.getenv("LLAMA_CTX", "8192")),
                n_threads=int(os.getenv("LLAMA_THREADS", str(os.cpu_count() or 4))),
                verbose=False,
            )
        return _llm_instance


def get_schema() -> Dict[str, Any]:
    global _schema_cache
    if _schema_cache is None:
        with open(SCHEMA_PATH, "r") as f:
            _schema_cache = json.load(f)
    return _schema_cache


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
    examples.append({
        "nl": "List total order amount per user",
        "sql": "SELECT u.id, u.name, SUM(o.amount) AS total_amount FROM users u JOIN orders o ON o.user_id = u.id GROUP BY u.id, u.name ORDER BY total_amount DESC;"
    })
    examples.append({
        "nl": "Orders in the last 30 days for user with email alice@example.com",
        "sql": "SELECT o.id, o.order_number, o.amount, o.status, o.order_date FROM orders o JOIN users u ON u.id = o.user_id WHERE u.email = 'alice@example.com' AND o.order_date >= DATE_SUB(CURRENT_DATE, INTERVAL 30 DAY) ORDER BY o.order_date DESC;"
    })
    examples.append({
        "nl": "Count of orders by status",
        "sql": "SELECT status, COUNT(*) AS cnt FROM orders GROUP BY status ORDER BY cnt DESC;"
    })
    return examples


def build_prompt(nl: str, schema: Dict[str, Any], few_shots: Optional[List[Dict[str, str]]] = None) -> str:
    schema_text = json.dumps(schema, indent=2)
    if not few_shots:
        few_shots = build_few_shots(schema)
    shots = ""
    for ex in few_shots:
        shots += f"User: {ex['nl']}\nAssistant:\n{ex['sql']}\n\n"
    prompt = f"""You are a senior SQL engineer generating MySQL SQL. Use ONLY the provided schema and relationships. Output ONLY a single SQL query without explanations or markdown.

Schema JSON:
{schema_text}

Rules:
- Use only tables and columns that exist in the schema.
- Prefer explicit JOINs using relationships.
- Return SELECT queries only, safe and deterministic.
- Do not wrap SQL in code fences.
- End with a semicolon.

{shots}User: {nl}
Assistant:
"""
    return prompt


def run_llm(prompt: str) -> str:
    llm = get_llm()
    out = llm(
        prompt=prompt,
        temperature=0.0,
        max_tokens=int(os.getenv("LLAMA_MAX_TOKENS", "256")),
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
    return {"status": "ok", "model_path": MODEL_PATH, "schema_path": SCHEMA_PATH}


@app.get("/schema")
def schema_endpoint():
    return get_schema()


@app.post("/nl2sql")
def nl2sql_endpoint(payload: NLQuery):
    schema = get_schema()
    prompt = build_prompt(payload.query, schema, payload.few_shots)
    sql = run_llm(prompt)
    analysis = validate_sql(sql, schema)
    return {
        "sql": sql,
        **analysis
    }


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