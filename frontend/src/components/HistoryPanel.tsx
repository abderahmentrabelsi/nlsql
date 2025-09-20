import { Card, CardContent, CardHeader } from './ui/card';
import { Button } from './ui/button';

export type HistoryEntry = {
  q: string;
  sql?: string;
  exec?: { columns: string[]; rows: Record<string, any>[]; count: number; sql: string };
  resp?: {
    sql: string;
    used_tables: string[];
    invalid_tables: string[];
    invalid_columns: { table: string; column: string }[];
    suggestions: { tables: Record<string, string[]>; columns: Record<string, string[]> };
    valid: boolean;
    schema_source?: string;
    sliced_tables?: string[];
  };
  ts: number;
};

export default function HistoryPanel({
  items,
  onSelectSnapshot,
  onClear,
}: {
  items: HistoryEntry[];
  onSelectSnapshot: (entry: HistoryEntry) => void;
  onClear: () => void;
}) {
  return (
    <Card className="sticky top-4 max-h-[70vh] overflow-hidden">
      <CardHeader className="flex items-center justify-between">
        <div className="font-medium">History</div>
        <Button variant="outline" size="sm" onClick={onClear}>Clear</Button>
      </CardHeader>
      <CardContent className="overflow-y-auto max-h-[62vh] pr-1">
        {items.length === 0 ? (
          <div className="text-sm text-muted-foreground">No history yet.</div>
        ) : (
          <div className="space-y-2">
            {items.map((e, i) => (
              <button
                key={i}
                onClick={() => onSelectSnapshot(e)}
                className="w-full text-left text-sm rounded-xl border bg-white/5 hover:bg-white/10 px-3 py-2 transition-colors"
                title={e.q}
              >
                <div className="font-medium line-clamp-2">{e.q}</div>
                <div className="mt-1 text-xs text-muted-foreground flex items-center gap-2">
                  <span>{new Date(e.ts).toLocaleString()}</span>
                  {typeof e.exec?.count === 'number' && <span>• {e.exec.count} rows</span>}
                  {e.resp && (
                    <span className={e.resp.valid ? 'text-green-400' : 'text-amber-400'}>
                      • {e.resp.valid ? 'valid' : 'needs fix'}
                    </span>
                  )}
                </div>
              </button>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}