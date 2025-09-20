import React, { createContext, useCallback, useContext, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { AnimatePresence, motion } from 'framer-motion';

type ToastVariant = 'default' | 'success' | 'error' | 'warning';
type ToastItem = { id: string; title?: string; description?: string; variant?: ToastVariant; ttl?: number };

type ToastContextType = {
  toast: (t: Omit<ToastItem, 'id'>) => void;
};
const ToastContext = createContext<ToastContextType | null>(null);

export function useToast(): ToastContextType {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error('useToast must be used within ToastProvider');
  return ctx;
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const remove = useCallback((id: string) => setItems((xs) => xs.filter((t) => t.id !== id)), []);

  const toast = useCallback(
    (t: Omit<ToastItem, 'id'>) => {
      const id = cryptoRandomId();
      const ttl = typeof t.ttl === 'number' ? t.ttl : 4000;
      const item: ToastItem = { id, ...t };
      setItems((xs) => [...xs, item]);
      if (ttl > 0) setTimeout(() => remove(id), ttl);
    },
    [remove]
  );

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      {createPortal(<ToastViewport items={items} onClose={remove} />, document.body)}
    </ToastContext.Provider>
  );
}

function ToastViewport({ items, onClose }: { items: ToastItem[]; onClose: (id: string) => void }) {
  return (
    <div className="pointer-events-none fixed right-4 top-4 z-[9999] flex w-[360px] max-w-[90vw] flex-col gap-2">
      <AnimatePresence initial={false}>
        {items.map((t) => (
          <motion.div
            key={t.id}
            initial={{ opacity: 0, y: -12, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -12, scale: 0.98 }}
            transition={{ type: 'spring', duration: 0.35 }}
            className={cnToast(
              'pointer-events-auto rounded-xl border px-4 py-3 shadow-lg backdrop-blur',
              t.variant
            )}
          >
            <div className="flex items-start gap-3">
              <div className="min-w-0">
                {t.title && <div className="text-sm font-semibold">{t.title}</div>}
                {t.description && <div className="mt-0.5 text-sm text-muted-foreground break-words">{t.description}</div>}
              </div>
              <button
                onClick={() => onClose(t.id)}
                className="ml-auto rounded-md border border-white/10 px-2 text-xs text-muted-foreground hover:bg-white/10"
              >
                Close
              </button>
            </div>
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  );
}

function cnToast(base: string, variant: ToastVariant = 'default') {
  const map: Record<ToastVariant, string> = {
    default: 'bg-white/10 border-white/15 text-white',
    success: 'bg-emerald-500/15 border-emerald-400/30 text-emerald-100',
    error: 'bg-rose-500/15 border-rose-400/30 text-rose-100',
    warning: 'bg-amber-500/15 border-amber-400/30 text-amber-100',
  };
  return `${base} ${map[variant]}`;
}

function cryptoRandomId() {
  try {
    // @ts-ignore
    if (window && window.crypto && window.crypto.getRandomValues) {
      const buf = new Uint32Array(2);
      window.crypto.getRandomValues(buf);
      return `${buf[0].toString(16)}${buf[1].toString(16)}`;
    }
  } catch {}
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}