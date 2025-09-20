import { useEffect, useMemo, useRef, useState } from 'react';
import QueryInput from './QueryInput';
import MessageBubble, { ExecResult, NLSQLResponse } from './MessageBubble';
import HistoryPanel, { type HistoryEntry } from './HistoryPanel';
import { useToast } from './ui/toast';
import { AnimatePresence, motion } from 'framer-motion';

type UserMsg = { id: string; role: 'user'; text: string };
type SystemMsg = {
  id: string;
  role: 'system';
  sql?: string;
  exec?: ExecResult;
  resp?: NLSQLResponse;
  loading?: boolean;
  error?: string;
  // timing
  startedAt?: number;   // Date.now() when request started
  genMs?: number;       // generation/validation time
  execMs?: number;      // execution time
  totalMs?: number;     // total end-to-end
};
type ChatMsg = UserMsg | SystemMsg;

const DEFAULT_EXAMPLES = [
  'total order amount per user',
  'sum cost per user',
  'list orders last 7 days for alice@example.com',
  'count of orders by status',
  'top 5 users by total amount',
];

const HISTORY_KEY = 'nlsql_history_v2';
const LEGACY_HISTORY_KEY = 'nlsql_history';
const HISTORY_LIMIT = 50;

function uniqueHeadFirst(items: string[]) {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const s of items) {
    const t = (s || '').trim();
    if (!t) continue;
    if (!seen.has(t)) {
      seen.add(t);
      out.push(t);
    }
    if (out.length >= HISTORY_LIMIT) break;
  }
  return out;
}

function makeId() {
  // crypto.randomUUID fallback for older browsers
  try {
    // @ts-ignore
    if (window && window.crypto && typeof window.crypto.randomUUID === 'function') {
      // @ts-ignore
      return window.crypto.randomUUID();
    }
  } catch {}
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}

export default function ChatPanel({ showHistory = false, onCloseHistory }: { showHistory?: boolean; onCloseHistory?: () => void }) {
  const [messages, setMessages] = useState<ChatMsg[]>([]);
  const [history, setHistory] = useState<HistoryEntry[]>([]);
  const listRef = useRef<HTMLDivElement>(null);

  // Load history with migration from legacy (string[]) and sanitize bad entries
  useEffect(() => {
    try {
      const parsedV2 = safeParseArray(localStorage.getItem(HISTORY_KEY));
      const parsedLegacy = safeParseArray(localStorage.getItem(LEGACY_HISTORY_KEY));
      const v2: HistoryEntry[] = Array.isArray(parsedV2)
        ? parsedV2
            .filter((e: any) => e && typeof e.q === 'string')
            .map((e: any) => sanitizeEntry(e))
        : [];
      const legacyStrings: string[] =
        Array.isArray(parsedLegacy) && parsedLegacy.every((x: any) => typeof x === 'string')
          ? parsedLegacy
          : [];
      const legacyEntries: HistoryEntry[] = legacyStrings.map((q) => ({
        q,
        ts: Date.now(),
      }));
      // Merge, preferring latest first and removing exact dupes (by q + sql signature)
      const merged = dedupeEntries([...v2, ...legacyEntries]).slice(0, HISTORY_LIMIT);
      setHistory(merged);
      // Persist merged to v2 and clear legacy
      localStorage.setItem(HISTORY_KEY, JSON.stringify(merged));
      localStorage.removeItem(LEGACY_HISTORY_KEY);
    } catch {
      // ignore
    }
  }, []);

  // autoscroll
  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight, behavior: 'smooth' });
  }, [messages]);

  const persistHistory = (items: HistoryEntry[]) => {
    const limited = items.slice(0, HISTORY_LIMIT);
    setHistory(limited);
    try {
      localStorage.setItem(HISTORY_KEY, JSON.stringify(limited));
    } catch {
      // ignore
    }
  };

  const dynamicExamples = useMemo(() => {
    const fromHistory = history.map((h) => h.q);
    return uniqueHeadFirst([...fromHistory, ...DEFAULT_EXAMPLES]).slice(0, 20);
  }, [history]);

  // toast
  const { toast } = useToast();

  const sendNL = async (text: string) => {
    const trimmed = text.trim();
    if (!trimmed) return;

    const id = makeId();
    setMessages((m) => [...m, { id, role: 'user', text: trimmed }]);

    const sysId = makeId();
    const startedAt = Date.now();
    const tStart = performance.now();
    setMessages((m) => [...m, { id: sysId, role: 'system', loading: true, startedAt }]);

    try {
      const t0 = performance.now();
      const gen = await fetch('/api/v1/nl2sql', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: trimmed }),
      });
      if (!gen.ok) {
        const txt = await gen.text().catch(() => '');
        throw new Error(txt || `Backend responded ${gen.status}`);
      }
      const resp: NLSQLResponse = await gen.json().catch(() => {
        throw new Error('Invalid JSON from backend');
      });
      const genMs = performance.now() - t0;

      // Auto-execute if valid
      let exec: ExecResult | undefined;
      let execMs: number | undefined;
      if (resp?.valid && resp?.sql) {
        const t1 = performance.now();
        const ex = await fetch('/api/v1/nl2sql/execute', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ sql: resp.sql }),
        });
        if (!ex.ok) {
          const txt = await ex.text().catch(() => '');
          throw new Error(txt || `Execute failed ${ex.status}`);
        }
        exec = await ex.json().catch(() => {
          throw new Error('Invalid JSON from execute');
        });
        execMs = performance.now() - t1;
      }

      const totalMs = performance.now() - tStart;

      // Update chat
      setMessages((m) =>
        m.map((mm) =>
          mm.id === sysId
            ? {
                id: sysId,
                role: 'system',
                sql: resp?.sql,
                exec,
                resp,
                loading: false,
                startedAt,
                genMs,
                execMs,
                totalMs,
              }
            : mm
        )
      );

      // Snapshot into history (no re-exec on recall)
      const entry: HistoryEntry = {
        q: trimmed,
        sql: resp?.sql,
        exec: exec ? sanitizeExec(exec) : undefined,
        resp: resp ? sanitizeResp(resp) : undefined,
        ts: Date.now(),
      };
      persistHistory(dedupeEntries([entry, ...history]));
    } catch (e: any) {
      const totalMs = performance.now() - tStart;
      setMessages((m) =>
        m.map((mm) =>
          mm.id === sysId
            ? { id: sysId, role: 'system', error: String(e), loading: false, startedAt, totalMs }
            : mm
        )
      );
      toast({
        title: 'Backend unavailable',
        description: typeof e?.message === 'string' ? e.message : 'Please start the API servers and try again.',
        variant: 'error',
      });
    }
  };

  const onRerun = async (id: string, sql: string) => {
    const startedAt = Date.now();
    const tStart = performance.now();
    setMessages((m) => m.map((mm) => (mm.id === id ? { ...(mm as SystemMsg), loading: true, startedAt } : mm)));
    try {
      const t1 = performance.now();
      const ex = await fetch('/api/v1/nl2sql/execute', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sql }),
      });
      if (!ex.ok) {
        const txt = await ex.text().catch(() => '');
        throw new Error(txt || `Execute failed ${ex.status}`);
      }
      const exec: ExecResult = await ex.json().catch(() => {
        throw new Error('Invalid JSON from execute');
      });
      const execMs = performance.now() - t1;
      const totalMs = performance.now() - tStart;
      setMessages((m) =>
        m.map((mm) => (mm.id === id ? { ...(mm as SystemMsg), sql, exec, loading: false, execMs, totalMs } : mm))
      );
    } catch (e: any) {
      const totalMs = performance.now() - tStart;
      setMessages((m) =>
        m.map((mm) => (mm.id === id ? { ...(mm as SystemMsg), loading: false, error: String(e), totalMs } : mm))
      );
      toast({
        title: 'Execution failed',
        description: typeof e?.message === 'string' ? e.message : 'Start the backend and try again.',
        variant: 'error',
      });
    }
  };

  const onRepair = async (id: string, sql: string, corrections: any) => {
    const startedAt = Date.now();
    const tStart = performance.now();
    setMessages((m) => m.map((mm) => (mm.id === id ? { ...(mm as SystemMsg), loading: true, startedAt } : mm)));
    try {
      const t0 = performance.now();
      const rep = await fetch('/api/v1/nl2sql/repair', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sql, corrections }),
      });
      if (!rep.ok) {
        const txt = await rep.text().catch(() => '');
        throw new Error(txt || `Repair failed ${rep.status}`);
      }
      const resp: NLSQLResponse = await rep.json().catch(() => {
        throw new Error('Invalid JSON from repair');
      });
      const genMs = performance.now() - t0;

      let exec: ExecResult | undefined;
      let execMs: number | undefined;
      if (resp?.valid && resp?.sql) {
        const t1 = performance.now();
        const ex = await fetch('/api/v1/nl2sql/execute', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ sql: resp.sql }),
        });
        if (!ex.ok) {
          const txt = await ex.text().catch(() => '');
          throw new Error(txt || `Execute failed ${ex.status}`);
        }
        exec = await ex.json().catch(() => {
          throw new Error('Invalid JSON from execute');
        });
        execMs = performance.now() - t1;
      }

      const totalMs = performance.now() - tStart;

      setMessages((m) =>
        m.map((mm) =>
          mm.id === id
            ? { ...(mm as SystemMsg), sql: resp?.sql, exec, resp, loading: false, startedAt, genMs, execMs, totalMs }
            : mm
        )
      );
    } catch (e: any) {
      const totalMs = performance.now() - tStart;
      setMessages((m) =>
        m.map((mm) => (mm.id === id ? { ...(mm as SystemMsg), loading: false, error: String(e), totalMs } : mm))
      );
      toast({
        title: 'Repair failed',
        description: typeof e?.message === 'string' ? e.message : 'Start the backend and try again.',
        variant: 'error',
      });
    }
  };

  // Show snapshot without re-executing; handle legacy/minimal entries safely
  // Clicking history should resend automatically (no snapshot render)
  const onSelectSnapshot = (entry: HistoryEntry) => {
    if (!entry || typeof entry.q !== 'string') return;
    sendNL(entry.q);
  };

  const clearHistory = () => persistHistory([]);

  return (
    <div className="relative">
      {/* Mobile: slide-in drawer (overlay) */}
      <AnimatePresence>
        {showHistory && (
          <motion.aside
            initial={{ x: -360, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            exit={{ x: -360, opacity: 0 }}
            transition={{ type: 'spring', duration: 0.35 }}
            className="fixed top-28 left-0 z-50 w-[85vw] max-w-[360px] px-4 lg:hidden"
          >
            <HistoryPanel items={history} onSelectSnapshot={onSelectSnapshot} onClear={clearHistory} />
          </motion.aside>
        )}
      </AnimatePresence>
      <AnimatePresence>
        {showHistory && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-40 bg-black/30 backdrop-blur-sm lg:hidden"
            onClick={onCloseHistory}
          />
        )}
      </AnimatePresence>

      {/* Desktop: fixed sidebar at page left (ChatGPT-like). App wrapper shifts content via lg:ml-[340px]. */}
      {showHistory && (
        <aside className="hidden lg:block fixed top-28 left-0 bottom-4 z-50 w-[320px]">
          <div className="h-full overflow-y-auto pl-4 pr-2">
            <HistoryPanel items={history} onSelectSnapshot={onSelectSnapshot} onClear={clearHistory} />
          </div>
        </aside>
      )}

      {/* Chat column (centered). No extra grid/margins here; App.tsx adds lg:ml-[340px] when sidebar visible. */}
      <main>
        <div className="max-w-3xl mx-auto px-4">
          <div
            ref={listRef}
            className="rounded-2xl border bg-card/60 backdrop-blur p-4 h-[70vh] overflow-y-auto"
          >
            <div className="space-y-4">
              {messages.map((m) => (
                <MessageBubble key={m.id} msg={m as any} onRerun={onRerun} onRepair={onRepair} />
              ))}
            </div>
          </div>
          <div className="mt-4">
            <QueryInput onSend={sendNL} examples={dynamicExamples} />
          </div>
        </div>
      </main>
    </div>
  );
}

// Helpers

function safeParseArray(raw: string | null): any[] | null {
  if (!raw) return null;
  try {
    const v = JSON.parse(raw);
    return Array.isArray(v) ? v : null;
  } catch {
    return null;
  }
}

function sanitizeExec(exec: any): ExecResult | undefined {
  if (!exec || !Array.isArray(exec.columns) || !Array.isArray(exec.rows)) return undefined;
  const cols = exec.columns.filter((c: any) => typeof c === 'string');
  const rows = exec.rows.filter((r: any) => r && typeof r === 'object');
  const count = typeof exec.count === 'number' ? exec.count : rows.length;
  const sql = typeof exec.sql === 'string' ? exec.sql : '';
  return { columns: cols, rows, count, sql };
}

function sanitizeResp(resp: any): NLSQLResponse | undefined {
  if (!resp || typeof resp !== 'object') return undefined;
  return {
    sql: typeof resp.sql === 'string' ? resp.sql : '',
    used_tables: Array.isArray(resp.used_tables) ? resp.used_tables : [],
    invalid_tables: Array.isArray(resp.invalid_tables) ? resp.invalid_tables : [],
    invalid_columns: Array.isArray(resp.invalid_columns) ? resp.invalid_columns : [],
    suggestions: resp.suggestions && typeof resp.suggestions === 'object' ? resp.suggestions : { tables: {}, columns: {} },
    valid: Boolean(resp.valid),
    schema_source: typeof resp.schema_source === 'string' ? resp.schema_source : undefined,
    sliced_tables: Array.isArray(resp.sliced_tables) ? resp.sliced_tables : undefined,
  };
}

function sanitizeEntry(e: any): HistoryEntry {
  return {
    q: typeof e.q === 'string' ? e.q : '',
    sql: typeof e.sql === 'string' ? e.sql : undefined,
    exec: e.exec ? sanitizeExec(e.exec) : undefined,
    resp: e.resp ? sanitizeResp(e.resp) : undefined,
    ts: typeof e.ts === 'number' ? e.ts : Date.now(),
  };
}

function dedupeEntries(entries: HistoryEntry[]): HistoryEntry[] {
  const seen = new Set<string>();
  const out: HistoryEntry[] = [];
  for (const e of entries) {
    const key = `${e.q}__${e.sql ?? e.resp?.sql ?? ''}`;
    if (!seen.has(key)) {
      seen.add(key);
      out.push(e);
    }
    if (out.length >= HISTORY_LIMIT) break;
  }
  return out;
}
