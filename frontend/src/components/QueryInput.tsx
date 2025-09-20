import { useMemo, useState } from 'react';
import { Input } from './ui/input';
import { Button } from './ui/button';
import { cn } from '../lib/utils';

export default function QueryInput({ onSend, examples }: { onSend: (q: string) => void; examples: string[] }) {
  const [q, setQ] = useState('');
  const [open, setOpen] = useState(false);

  const filtered = useMemo(() => {
    const v = q.toLowerCase();
    if (!v) return examples.slice(0, 5);
    return examples.filter((e) => e.toLowerCase().includes(v)).slice(0, 5);
  }, [q, examples]);
  const hasFiltered = filtered.length > 0;

  return (
    <div className="max-w-3xl mx-auto">
      <div className="flex gap-2">
        <Input
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && q.trim()) {
              onSend(q.trim());
              setOpen(false);
              setQ('');
            }
          }}
          placeholder="Ask in natural language…"
        />
        <Button onClick={() => q.trim() && (onSend(q.trim()), setQ(''))}>Send</Button>
      </div>
      <div className={cn('mt-2 rounded-xl border bg-card p-2 text-sm', open && hasFiltered ? 'block' : 'hidden')}>
        {hasFiltered && <div className="text-muted-foreground mb-1">Examples</div>}
        <div className="flex flex-wrap gap-2">
          {filtered.map((e) => (
            <button
              key={e}
              onClick={() => (onSend(e), setQ(''), setOpen(false))}
              className="px-3 py-1 rounded-full bg-accent hover:opacity-90"
            >
              {e}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}
