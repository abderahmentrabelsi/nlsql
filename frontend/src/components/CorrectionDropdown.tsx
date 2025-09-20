import { useMemo, useState } from 'react';
import { Select } from './ui/select';
import { Button } from './ui/button';

type Suggestions = { tables: Record<string, string[]>; columns: Record<string, string[]> };
type InvalidCol = { table: string; column: string };

export default function CorrectionDropdown({
  sql,
  invalidTables,
  invalidColumns,
  suggestions,
  onApply
}: {
  sql: string;
  invalidTables: string[];
  invalidColumns: InvalidCol[];
  suggestions: Suggestions;
  onApply: (corrections: any) => void;
}) {
  const [tableFixes, setTableFixes] = useState<Record<string, string>>({});
  const [columnFixes, setColumnFixes] = useState<Record<string, string>>({});

  const has = invalidTables.length || invalidColumns.length;

  const canApply = useMemo(
    () =>
      invalidTables.every((t) => tableFixes[t]) ||
      invalidColumns.every((ic) => columnFixes[ic.table ? `${ic.table}.${ic.column}` : ic.column]),
    [invalidTables, invalidColumns, tableFixes, columnFixes]
  );

  if (!has) return null;

  return (
    <div className="rounded-xl border bg-card p-3 space-y-3">
      <div className="text-sm font-medium">Fix invalid references</div>
      {invalidTables.map((t) => (
        <Select
          key={t}
          label={`Table "${t}" not found`}
          value={tableFixes[t] || ''}
          onChange={(e) => setTableFixes((s) => ({ ...s, [t]: e.target.value }))}
        >
          <option value="" disabled>Choose replacement…</option>
          {(suggestions.tables[t] || []).map((opt) => (
            <option key={opt} value={opt}>{opt}</option>
          ))}
        </Select>
      ))}
      {invalidColumns.map((ic, idx) => {
        const key = ic.table ? `${ic.table}.${ic.column}` : ic.column;
        const opts = ic.table && suggestions.columns[`${ic.table}.${ic.column}`]
          ? suggestions.columns[`${ic.table}.${ic.column}`]
          : suggestions.columns[ic.column] || [];
        return (
          <Select
            key={key + idx}
            label={`Column "${key}" not found`}
            value={columnFixes[key] || ''}
            onChange={(e) => setColumnFixes((s) => ({ ...s, [key]: e.target.value }))}
          >
            <option value="" disabled>Choose replacement…</option>
            {opts.map((opt) => (
              <option key={opt} value={ic.table ? `${ic.table}.${opt}` : opt}>{opt}</option>
            ))}
          </Select>
        );
      })}
      <div className="pt-2">
        <Button
          onClick={() => onApply({ tables: tableFixes, columns: columnFixes })}
          disabled={!canApply}
        >
          Apply corrections & Re-run
        </Button>
      </div>
    </div>
  );
}
