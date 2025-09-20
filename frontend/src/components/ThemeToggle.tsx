import { useEffect, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Moon, Sun } from 'lucide-react';

export default function ThemeToggle({ onChange }: { onChange?: (dark: boolean) => void }) {
  const [dark, setDark] = useState<boolean>(() => {
    if (typeof window === 'undefined') return true;
    const saved = localStorage.getItem('theme');
    return saved ? saved !== 'light' : true;
  });

  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark);
    localStorage.setItem('theme', dark ? 'dark' : 'light');
    onChange?.(dark);
  }, [dark, onChange]);

  return (
    <button
      aria-label="Toggle theme"
      onClick={() => setDark((v) => !v)}
      className="relative inline-flex items-center gap-2 rounded-xl border border-white/15 bg-white/5 px-3 py-2 text-sm hover:bg-white/10 transition-colors"
    >
      <div className="relative w-5 h-5">
        <AnimatePresence initial={false}>
          {dark ? (
            <motion.span
              key="moon"
              initial={{ rotate: -90, opacity: 0, scale: 0.6 }}
              animate={{ rotate: 0, opacity: 1, scale: 1 }}
              exit={{ rotate: 90, opacity: 0, scale: 0.6 }}
              transition={{ type: 'spring', duration: 0.35 }}
              className="absolute inset-0"
            >
              <Moon className="h-5 w-5 text-fuchsia-300" />
            </motion.span>
          ) : (
            <motion.span
              key="sun"
              initial={{ rotate: 90, opacity: 0, scale: 0.6 }}
              animate={{ rotate: 0, opacity: 1, scale: 1 }}
              exit={{ rotate: -90, opacity: 0, scale: 0.6 }}
              transition={{ type: 'spring', duration: 0.35 }}
              className="absolute inset-0"
            >
              <Sun className="h-5 w-5 text-amber-400" />
            </motion.span>
          )}
        </AnimatePresence>
      </div>
      <span className="hidden sm:inline">{dark ? 'Dark' : 'Light'}</span>
    </button>
  );
}