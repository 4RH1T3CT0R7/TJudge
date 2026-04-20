import { useState, useEffect, useCallback, useRef } from 'react';
import { Link } from 'react-router-dom';
import { SpaceInvader } from '../components/SpaceInvader';
import type { InvaderPose } from '../components/SpaceInvader';
import { useRapidClicks, useDoubleClickText } from '../hooks/useEasterEggs';

const CYCLING_SPEECHES = [
  '// 404',
  '// заблудились',
  '// segfault in navigation',
  '// git checkout -- reality',
  '// null pointer',
  '// page not found',
];

const SLOT_CODES = [
  { code: '418', speech: '// я чайник' },
  { code: '451', speech: '// цензура!' },
  { code: '508', speech: '// бесконечный цикл!' },
];

export function NotFound() {
  const speechIndexRef = useRef(0);
  const [displayCode, setDisplayCode] = useState('404');
  const [invaderPose, setInvaderPose] = useState<InvaderPose>('idle');
  const [slotRolling, setSlotRolling] = useState(false);
  const [invaderSpeech, setInvaderSpeech] = useState<string | null>(CYCLING_SPEECHES[0]);

  // Cycle speeches
  useEffect(() => {
    const interval = setInterval(() => {
      speechIndexRef.current = (speechIndexRef.current + 1) % CYCLING_SPEECHES.length;
      setInvaderSpeech(CYCLING_SPEECHES[speechIndexRef.current]);
    }, 4000);
    return () => clearInterval(interval);
  }, []);

  // 10 rapid clicks easter egg
  const handleRapidClick = useRapidClicks(10, useCallback(() => {
    setInvaderPose('fly');
    setInvaderSpeech('// улетаю!');
    setTimeout(() => {
      setInvaderPose('teleport');
      setInvaderSpeech('// я вернулся!');
      setTimeout(() => {
        setInvaderPose('idle');
        setInvaderSpeech(CYCLING_SPEECHES[0]);
      }, 1500);
    }, 2000);
  }, []));

  // Двойной клик по 404 - слот-машина
  const handleDoubleClick404 = useDoubleClickText(useCallback(() => {
    if (slotRolling) return;
    setSlotRolling(true);
    const slot = SLOT_CODES[Math.floor(Math.random() * SLOT_CODES.length)];
    setDisplayCode(slot.code);
    setInvaderSpeech(slot.speech);
    setInvaderPose('dizzy');
    setTimeout(() => {
      setDisplayCode('404');
      setInvaderPose('idle');
      setInvaderSpeech(CYCLING_SPEECHES[0]);
      setSlotRolling(false);
    }, 3000);
  }, [slotRolling]));

  return (
    <div className="flex flex-col items-center justify-center min-h-[70vh] py-12 px-4 relative overflow-hidden">
      {/* Glow orbs */}
      <div
        className="absolute top-1/4 -left-20 w-72 h-72 rounded-full opacity-15 blur-3xl pointer-events-none"
        style={{ background: 'radial-gradient(circle, rgba(139,92,246,0.5), transparent 70%)' }}
      />
      <div
        className="absolute bottom-1/4 -right-20 w-72 h-72 rounded-full opacity-10 blur-3xl pointer-events-none"
        style={{ background: 'radial-gradient(circle, rgba(74,222,128,0.4), transparent 70%)' }}
      />

      <div style={{ zIndex: 60, position: 'relative' }} onClick={handleRapidClick}>
        <SpaceInvader
          size="lg"
          className="mb-8"
          interactive
          controlledPose={invaderPose !== 'idle' ? invaderPose : undefined}
          speechBubble={invaderSpeech}
        />
      </div>

      <h1
        className="text-7xl md:text-9xl font-extrabold mb-4 select-none cursor-pointer"
        style={{
          background: displayCode !== '404'
            ? 'linear-gradient(135deg, #4ade80, #22c55e, #16a34a, #4ade80)'
            : 'linear-gradient(135deg, #c4b5fd, #8b5cf6, #6d28d9, #c4b5fd)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          backgroundClip: 'text',
          transition: 'all 0.3s ease',
        }}
        onDoubleClick={handleDoubleClick404}
      >
        {displayCode}
      </h1>

      <p className="text-xl text-gray-400 mb-2">
        Страница не найдена
      </p>
      <p className="text-sm text-gray-500 mb-8">
        Этот инвейдер тоже потерялся в космосе
      </p>

      <Link to="/" className="btn btn-primary">
        Вернуться на главную
      </Link>
    </div>
  );
}
