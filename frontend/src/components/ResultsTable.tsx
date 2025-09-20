import { useMemo, useState } from 'react';
import { Card, CardContent, CardHeader } from './ui/card';

export default function ResultsTable({ columns, rows }: { columns: string[]; rows: Record<string, any>[] }) {
  const [page, setPage] = useState(1);
  const pageSize = 10;
  const total = rows.length;
  const pages = Math.max(1, Math.ceil(total / pageSize));

  const slice = useMemo(() => rows.slice((page - 1) * pageSize, page * pageSize), [rows, page]);

  return (
    <Card>
      <CardHeader className="flex items-center justify-between">
        <div className="font-medium">Results</div>
        <div className="text-xs text-muted-foreground">
          {total} rows • Page {page}/{pages}
        </div>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b">
              {columns.map((c) => (
                <th key={c} className="text-left py-2 pr-4 font-semibold">{c}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {slice.map((r, i) => (
              <tr key={i} className="border-b last:border-0">
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
