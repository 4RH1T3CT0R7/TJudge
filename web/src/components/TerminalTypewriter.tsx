import { useState, useEffect, useRef, useCallback } from 'react';

interface Phrase {
  text: string;
  color: string;
}

const PHRASES: Phrase[] = [
  { text: 'tit_for_tat.py',             color: '#fb7185' },
  { text: 'nash_equilibrium()',          color: '#a78bfa' },
  { text: 'cooperate || defect',         color: '#4ade80' },
  { text: 'Дилемма заключённого',        color: '#60a5fa' },
  { text: 'strategy.optimize()',         color: '#fbbf24' },
  { text: 'payoff_matrix[i][j]',         color: '#22d3ee' },
  { text: 'Равновесие Нэша',             color: '#f472b6' },
  { text: 'while !dominated { play() }', color: '#a78bfa' },
  { text: 'Оптимальность по Парето',     color: '#34d399' },
];

type Phase = 'typing' | 'paused' | 'deleting' | 'waiting';

export function TerminalTypewriter() {
  const [phraseIndex, setPhraseIndex] = useState(0);
  const [displayedText, setDisplayedText] = useState('');
  const [phase, setPhase] = useState<Phase>('typing');
  const charIndex = useRef(0);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const currentPhrase = PHRASES[phraseIndex];

  const tick = useCallback(() => {
    switch (phase) {
      case 'typing': {
        const full = currentPhrase.text;
        if (charIndex.current < full.length) {
          charIndex.current++;
          setDisplayedText(full.slice(0, charIndex.current));
          timerRef.current = setTimeout(tick, 80);
        } else {
          setPhase('paused');
        }
        break;
      }
      case 'paused':
        timerRef.current = setTimeout(() => setPhase('deleting'), 2000);
        break;
      case 'deleting': {
        if (charIndex.current > 0) {
          charIndex.current--;
          setDisplayedText(currentPhrase.text.slice(0, charIndex.current));
          timerRef.current = setTimeout(tick, 40);
        } else {
          setPhase('waiting');
        }
        break;
      }
      case 'waiting':
        timerRef.current = setTimeout(() => {
          setPhraseIndex((prev) => (prev + 1) % PHRASES.length);
          charIndex.current = 0;
          setDisplayedText('');
          setPhase('typing');
        }, 300);
        break;
    }
  }, [phase, currentPhrase]);

  useEffect(() => {
    timerRef.current = setTimeout(tick, phase === 'typing' ? 80 : 0);
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [tick, phase]);

  return (
    <div className="w-full max-w-2xl mx-auto">
      {/* Terminal chrome — macOS dots */}
      <div className="bg-gray-800 rounded-t-xl px-4 py-2.5 flex items-center gap-2 border border-gray-700/50 border-b-0">
        <div className="w-3 h-3 rounded-full bg-red-500/80" />
        <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
        <div className="w-3 h-3 rounded-full bg-green-500/80" />
        <span className="ml-3 text-xs text-gray-500 font-mono">tjudge ~ strategy</span>
      </div>
      {/* Terminal body */}
      <div
        className="bg-gray-900/80 rounded-b-xl px-6 py-8 border border-gray-700/50 border-t-0"
        style={{ fontFamily: "'JetBrains Mono', monospace" }}
      >
        <div className="flex items-center text-lg min-h-[1.75rem]">
          <span className="text-green-400 mr-3">$</span>
          <span style={{ color: currentPhrase.color }}>{displayedText}</span>
          <span className="terminal-cursor ml-0.5 text-gray-400">|</span>
        </div>
      </div>
    </div>
  );
}
