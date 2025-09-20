import { useEffect, useState } from 'react';
import ChatPanel from './components/ChatPanel';
import Starfield from './components/Starfield';
import ThemeToggle from './components/ThemeToggle';
import { Github, History as HistoryIcon } from 'lucide-react';

export default function App() {
  // Track theme so Starfield can adapt to light/dark
  const [dark, setDark] = useState<boolean>(() => {
    const saved = localStorage.getItem('theme');
    return saved ? saved !== 'light' : true;
  });

  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark);
  }, [dark]);

  const [showHistory, setShowHistory] = useState(false);

  return (
    <div className={`relative min-h-screen bg-background text-foreground ${showHistory ? 'lg:ml-[340px]' : ''}`}>
      <Starfield count={240} dark={dark} />

      <header className="max-w-6xl mx-auto px-4 py-6 flex items-center justify-between relative z-10">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            <div className="inline-flex items-center rounded-2xl border border-white/15 bg-white/5 px-3 py-2 backdrop-blur supports-[backdrop-filter]:bg-white/10">
              <span className="text-2xl font-bold tracking-tight bg-gradient-to-r from-violet-400 via-fuchsia-400 to-cyan-400 bg-clip-text text-transparent">
                SQLCoder Studio
              </span>
            </div>
            <button
              onClick={() => setShowHistory((v) => !v)}
              className="inline-flex items-center gap-1 rounded-xl border border-white/15 bg-white/5 px-3 py-2 text-xs hover:bg-white/10 transition-colors"
              title="Toggle history"
            >
              <HistoryIcon className="h-3 w-3" />
              History
            </button>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <ThemeToggle onChange={setDark} />
          <a
            href="https://github.com/abderahmentrabelsi"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-2 rounded-xl border border-white/15 bg-white/5 px-3 py-2 text-sm hover:bg-white/10 transition-colors"
            title="Author: Abderahmen Trabelsi"
          >
            <Github className="h-4 w-4" />
            abderahmentrabelsi
          </a>
        </div>
      </header>

      <main className="relative z-10">
        <div className="max-w-6xl mx-auto px-4 pb-10">
          <ChatPanel showHistory={showHistory} onCloseHistory={() => setShowHistory(false)} />
        </div>
      </main>
    </div>
  );
}
