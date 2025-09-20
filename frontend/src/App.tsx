import { useEffect, useState } from 'react';
import ChatPanel from './components/ChatPanel';
import Starfield from './components/Starfield';
import ThemeToggle from './components/ThemeToggle';
import { Github } from 'lucide-react';

export default function App() {
  // Track theme so Starfield can adapt to light/dark
  const [dark, setDark] = useState<boolean>(() => {
    const saved = localStorage.getItem('theme');
    return saved ? saved !== 'light' : true;
  });

  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark);
  }, [dark]);

  return (
    <div className="relative min-h-screen bg-background text-foreground">
      <Starfield count={160} dark={dark} />

      <header className="max-w-6xl mx-auto px-4 py-6 flex items-center justify-between relative z-10">
        <h1 className="text-xl font-semibold bg-gradient-to-r from-fuchsia-400 via-purple-400 to-indigo-400 bg-clip-text text-transparent">
          NL→SQL Chat
        </h1>
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
          <ChatPanel />
        </div>
      </main>
    </div>
  );
}
