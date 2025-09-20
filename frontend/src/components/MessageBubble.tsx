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

export default function MessageBubble({
  msg,
  onRerun,
  onRepair
}: {
  msg: any;
  onRerun: (id: string, sql: string) => void;
  onRepair: (id: string, sql: string, corrections: any) => void;
}) {
  if (msg.role === 'user') {
    return (
      <div className="flex justify-end">
        <div className="max-w-[80%] rounded-2xl bg-primary text-primary-foreground px-4 py-3 shadow-lg">
          {msg.text}
        </div>
      </div>
    );
  }

  return (
    <div className="flex justify-start">
      <div className="w-full max-w-[80%] space-y-3">
        {msg.loading && (
          <div className="rounded-2xl border bg-card px-4 py-3 flex items-center gap-3">
            <Spinner /> <span className="text-sm text-muted-foreground">Thinking…</span>
          </div>
        )}
        {msg.error && (
          <div className="rounded-2xl border bg-card px-4 py-3 text-sm text-red-500">
            {msg.error}
          </div>
        )}
        {msg.exec && <ResultsTable columns={msg.exec.columns} rows={msg.exec.rows} />}
        {msg.sql && <SQLBlock id={msg.id} sql={msg.sql} loading={msg.loading} onRerun={onRerun} />}
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
    </div>
  );
}

function SQLBlock({
  id,
  sql,
  loading,
  onRerun
}: {
  id: string;
  sql: string;
  loading?: boolean;
  onRerun: (id: string, sql: string) => void;
}) {
  const [open, setOpen] = React.useState(false);
  const [edit, setEdit] = React.useState(false);
  const [value, setValue] = React.useState(sql);

  React.useEffect(() => setValue(sql), [sql]);

  return (
    <Card className="border bg-card/70">
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between pt-4">
          <div className="text-sm font-medium">Generated SQL</div>
          <div className="flex items-center gap-2">
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
                <pre className="p-3 overflow-auto"><code>{sql}</code></pre>
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
