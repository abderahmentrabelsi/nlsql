import { useMemo, useState } from 'react';
import { Card, CardContent, CardHeader } from './ui/card';
import { Button } from './ui/button';

export default function ResultsTable({ columns, rows }: { columns: string[]; rows: Record<string, any>[] }) {
  const [page, setPage] = useState(1);
  const pageSize = 10;
  const total = rows.length;
  const pages = Math.max(1, Math.ceil(total / pageSize));

  const slice = useMemo(() => rows.slice((page - 1) * pageSize, page * pageSize), [rows, page]);

  function toCSV(cols: string[], rws: Record<string, any>[]) {
    const head = cols.join(',');
    const body = rws
      .map((row) =>
        cols
          .map((c) => {
            const v = row[c] ?? '';
            const s = String(v).replace(/"/g, '""');
            return `"${s}"`;
          })
          .join(',')
      )
      .join('\n');
    return head + '\n' + body;
  }
  function downloadCSV(csv: string) {
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'results.csv';
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <Card>
      <CardHeader className="flex items-center justify-between">
        <div className="font-medium">Results</div>
        <div className="flex items-center gap-2">
          <div className="text-xs text-muted-foreground mr-1">
            {total} rows • Page {page}/{pages}
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              const csv = toCSV(columns, rows);
              navigator.clipboard.writeText(csv);
            }}
          >
            Copy CSV
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => downloadCSV(toCSV(columns, rows))}
          >
            Download CSV
          </Button>
        </div>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b uppercase tracking-wider text-xs text-muted-foreground">
              {columns.map((c) => (
                <th key={c} className="text-left py-2 pr-4">{c}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {slice.map((r, i) => (
              <tr key={i} className="border-b last:border-0 odd:bg-white/5 hover:bg-white/10 transition-colors">
                {columns.map((c) => (
                  <td key={c} className="py-2 pr-4 whitespace-nowrap">{String(r[c] ?? '')}</td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        <div className="flex items-center justify-end gap-2 pt-3">
          <button className="text-xs underline" onClick={() => setPage((p) => Math.max(1, p - 1))}>Prev</button>
          <button className="text-xs underline" onClick={() => setPage((p) => Math.min(pages, p + 1))}>Next</button>
        </div>
      </CardContent>
    </Card>
  );
}
