import * as React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import ResultsTable from './ResultsTable';
import { Button } from './ui/button';
import { Textarea } from './ui/textarea';
import { Card, CardContent } from './ui/card';
import { Spinner } from './ui/spinner';
import CorrectionDropdown from './CorrectionDropdown';

export type ExecResult = { columns: string[]; rows: Record<string, any>[]; count: number; sql: string };
export type NLSQLResponse = {
  sql: string;
  used_tables: string[];
  invalid_tables: string[];
  invalid_columns: { table: string; column: string }[];
  suggestions: { tables: Record<string, string[]>; columns: Record<string, string[]> };
  valid: boolean;
  schema_source?: string;
  sliced_tables?: string[];
};

function formatSecs(ms?: number): string {
  if (ms === undefined || ms === null) return '';
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return `${m}m ${rem}s`;
}

function TimingMeta({ genMs, execMs, totalMs }: { genMs?: number; execMs?: number; totalMs?: number }) {
  const Chip = ({ label }: { label: string }) => (
    <span className="inline-flex items-center rounded-full border border-white/15 bg-white/5 px-2 py-0.5 text-[10px] tracking-wide whitespace-nowrap leading-none">
      {label}
    </span>
  );
  return (
    <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
      {genMs != null && <Chip label={`Gen ${formatSecs(genMs)}`} />}
      {execMs != null && <Chip label={`Exec ${formatSecs(execMs)}`} />}
      {totalMs != null && <Chip label={`Total ${formatSecs(totalMs)}`} />}
    </div>
  );
}

function Dots() {
  const seq: number[] = [0.2, 1, 0.2];
  return (
    <span className="inline-flex items-center gap-1 ml-1 text-muted-foreground">
      <motion.span className="h-1.5 w-1.5 rounded-full bg-current" animate={{ opacity: seq }} transition={{ duration: 0.9, repeat: Infinity }} />
      <motion.span className="h-1.5 w-1.5 rounded-full bg-current" animate={{ opacity: seq }} transition={{ duration: 0.9, repeat: Infinity, delay: 0.15 }} />
      <motion.span className="h-1.5 w-1.5 rounded-full bg-current" animate={{ opacity: seq }} transition={{ duration: 0.9, repeat: Infinity, delay: 0.3 }} />
    </span>
  );
}

/* Lightweight SQL syntax highlighter (escape then wrap keywords/strings/numbers) */
function highlightSql(sql: string): string {
  const escape = (s: string) =>
    s.replace(/&/g, '&').replace(/</g, '<').replace(/>/g, '>');
  let out = escape(sql);

  // strings
  out = out.replace(/'[^']*'/g, (m) => `<span class="text-rose-300">${m}</span>`);
  out = out.replace(/"[^"]*"/g, (m) => `<span class="text-rose-300">${m}</span>`);
  // numbers
  out = out.replace(/\b\d+(?:\.\d+)?\b/g, (m) => `<span class="text-amber-300">${m}</span>`);
  // table.column
  out = out.replace(
    /\b([A-Za-z_][A-Za-z0-9_]*)\.(\*|[A-Za-z_][A-Za-z0-9_]*)\b/g,
    (_m, t, c) => `<span class="text-sky-300">${t}</span>.<span class="text-teal-300">${c}</span>`
  );
  // keywords
  const kws = [
    'SELECT','FROM','JOIN','LEFT','RIGHT','FULL','INNER','OUTER','ON','WHERE','GROUP','BY','HAVING','ORDER','ASC','DESC',
    'LIMIT','OFFSET','AS','AND','OR','NOT','IN','IS','NULL','DISTINCT','UNION','ALL','CASE','WHEN','THEN','ELSE','END','WITH'
  ];
  const re = new RegExp(`\\b(${kws.join('|')})\\b`, 'gi');
  out = out.replace(re, (m) => `<span class="text-violet-300 font-semibold">${m.toUpperCase()}</span>`);
  return out;
}

export default function MessageBubble({
  msg,
  onRerun,
  onRepair
}: {
  msg: any;
  onRerun: (id: string, sql: string) => void;
  onRepair: (id: string, sql: string, corrections: any) => void;
}) {
  const [elapsed, setElapsed] = React.useState<number>(0);
  React.useEffect(() => {
    if (msg.loading && msg.startedAt) {
      setElapsed(Date.now() - msg.startedAt);
      const h = setInterval(() => setElapsed(Date.now() - msg.startedAt), 1000);
      return () => clearInterval(h);
    } else {
      setElapsed(0);
    }
  }, [msg.loading, msg.startedAt]);

  if (msg.role === 'user') {
    return (
      <motion.div
        className="flex justify-end"
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.14 }}
      >
        <div className="max-w-[80%] rounded-2xl bg-primary text-primary-foreground px-4 py-3 shadow-lg">
          {msg.text}
        </div>
      </motion.div>
    );
  }

  return (
    <motion.div
      className="flex justify-start"
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.14 }}
    >
      <div className="w-full max-w-[80%] space-y-3">
        {msg.loading && (
          <div className="rounded-2xl border bg-card px-4 py-3 flex items-center gap-2">
            <Spinner />
            <span className="text-sm text-muted-foreground">Thinking… {formatSecs(elapsed)}</span>
            <Dots />
          </div>
        )}
        {msg.error && (
          <div className="rounded-2xl border bg-card px-4 py-3 text-sm text-red-500">
            {msg.error}
          </div>
        )}
        {msg.exec && <ResultsTable columns={msg.exec.columns} rows={msg.exec.rows} />}
        {msg.sql && (
          <SQLBlock
            id={msg.id}
            sql={msg.sql}
            loading={msg.loading}
            onRerun={onRerun}
            genMs={msg.genMs}
            execMs={msg.execMs}
            totalMs={msg.totalMs}
          />
        )}
        {msg.resp && !msg.resp.valid && (
          <CorrectionDropdown
            sql={msg.sql}
            invalidTables={msg.resp.invalid_tables}
            invalidColumns={msg.resp.invalid_columns}
            suggestions={msg.resp.suggestions}
            onApply={(corrections) => onRepair(msg.id, msg.sql, corrections)}
          />
        )}
      </div>
    </motion.div>
  );
}

function SQLBlock({
  id,
  sql,
  loading,
  onRerun,
  genMs,
  execMs,
  totalMs,
}: {
  id: string;
  sql: string;
  loading?: boolean;
  onRerun: (id: string, sql: string) => void;
  genMs?: number;
  execMs?: number;
  totalMs?: number;
}) {
  const [open, setOpen] = React.useState(false);
  const [edit, setEdit] = React.useState(false);
  const [value, setValue] = React.useState(sql);

  React.useEffect(() => setValue(sql), [sql]);

  return (
    <Card className="border bg-card/70">
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between pt-4">
          <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-3">
            <div className="text-sm font-medium">Generated SQL</div>
            <TimingMeta genMs={genMs} execMs={execMs} totalMs={totalMs} />
          </div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(value)}>
              Copy
            </Button>
            <Button variant="outline" size="sm" onClick={() => setEdit((v) => !v)}>
              {edit ? 'Cancel' : 'Edit & Re-run'}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setOpen((v) => !v)}>
              {open ? 'Hide' : 'Show'}
            </Button>
          </div>
        </div>
        <AnimatePresence initial={false}>
          {open && (
            <motion.div
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: 'auto', opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              transition={{ type: 'spring', duration: 0.3 }}
              className="overflow-hidden rounded-xl border bg-background"
            >
              {!edit ? (
                <pre className="p-3 overflow-auto">
                  <code dangerouslySetInnerHTML={{ __html: highlightSql(sql) }} />
                </pre>
              ) : (
                <div className="p-3 space-y-2">
                  <Textarea rows={6} value={value} onChange={(e) => setValue(e.target.value)} />
                  <Button disabled={loading} onClick={() => onRerun(id, value)}>
                    {loading ? 'Running…' : 'Run SQL'}
                  </Button>
                </div>
              )}
            </motion.div>
          )}
        </AnimatePresence>
      </CardContent>
    </Card>
  );
}
