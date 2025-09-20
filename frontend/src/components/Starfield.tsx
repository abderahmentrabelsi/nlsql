import { useMemo } from 'react';
import { motion } from 'framer-motion';

export default function Starfield({
  count = 160,
  dark = true,
}: {
  count?: number;
  dark?: boolean;
}) {
  const stars = useMemo(() => {
    const arr: {
      left: string;
      top: string;
      size: number;
      delay: number;
      dur: number;
      opacity: number;
    }[] = [];
    for (let i = 0; i < count; i++) {
      arr.push({
        left: Math.random() * 100 + '%',
        top: Math.random() * 100 + '%',
        size: Math.random() * 2 + 1, // 1px - 3px
        delay: Math.random() * 4, // 0-4s
        dur: 2 + Math.random() * 3, // 2-5s
        opacity: 0.3 + Math.random() * 0.7, // 0.3 - 1
      });
    }
    return arr;
  }, [count]);

  const bgClass = dark
    ? 'bg-[radial-gradient(ellipse_at_center,rgba(147,51,234,0.22),rgba(0,0,0,0.92))]'
    : 'bg-[radial-gradient(ellipse_at_center,rgba(147,51,234,0.12),rgba(255,255,255,0.92))]';

  const blob1 = dark ? 'bg-fuchsia-600/15' : 'bg-fuchsia-600/10';
  const blob2 = dark ? 'bg-indigo-500/10' : 'bg-indigo-500/8';
  const blob3 = dark ? 'bg-purple-400/10' : 'bg-purple-400/8';

  const starColor = dark ? 'rgba(255,255,255,0.9)' : 'rgba(0,0,0,0.8)';
  const starShadow = dark ? '0 0 6px rgba(255,255,255,0.6)' : '0 0 4px rgba(0,0,0,0.25)';
  const trailClass = dark
    ? 'bg-gradient-to-r from-transparent via-fuchsia-400/60 to-transparent'
    : 'bg-gradient-to-r from-transparent via-fuchsia-500/40 to-transparent';

  return (
    <div className="pointer-events-none fixed inset-0 z-0 overflow-hidden">
      {/* backdrop */}
      <div className={`absolute inset-0 ${bgClass}`} />

      {/* subtle blobs */}
      <div className={`absolute -top-24 -left-24 h-[50vh] w-[50vh] rounded-full ${blob1} blur-3xl`} />
      <div className={`absolute -bottom-32 -right-20 h-[60vh] w-[60vh] rounded-full ${blob2} blur-3xl`} />
      <div className={`absolute top-1/3 left-1/4 h-[30vh] w-[30vh] rounded-full ${blob3} blur-3xl`} />

      {/* stars */}
      {stars.map((s, i) => (
        <motion.div
          key={i}
          initial={{ opacity: 0 }}
          animate={{ opacity: [0, s.opacity, 0] }}
          transition={{ duration: s.dur, repeat: Infinity, ease: 'easeInOut', delay: s.delay }}
          className="absolute rounded-full"
          style={{
            left: s.left,
            top: s.top,
            width: s.size,
            height: s.size,
            backgroundColor: starColor,
            boxShadow: starShadow,
          }}
        />
      ))}

      {/* shooting trail */}
      <motion.div
        className={`absolute h-px w-40 ${trailClass}`}
        initial={{ opacity: 0, x: '-20%', y: '10%' }}
        animate={{ opacity: [0, 1, 0], x: ['-10%', '110%'], y: ['10%', '12%'] }}
        transition={{ duration: 6, repeat: Infinity, ease: 'easeInOut', delay: 1.5 }}
      />
    </div>
  );
}