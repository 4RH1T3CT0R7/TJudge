import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { useMotionPref } from '../../hooks/useMotionPref';

const FRAMES = '⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏';

// Брайлевый спиннер терминала: кадр раз в 80 мс, при reduced-motion стоит на месте.
// Без подписи скринридер слышит «Загрузка».
export function Spinner({ children, className = '' }: { children?: ReactNode; className?: string }) {
  const { reduced: reduceMotion } = useMotionPref();
  const [frame, setFrame] = useState(0);

  useEffect(() => {
    if (reduceMotion) return;
    const timer = setInterval(() => setFrame((f) => (f + 1) % FRAMES.length), 80);
    return () => clearInterval(timer);
  }, [reduceMotion]);

  return (
    <span role="status" className={`inline-flex items-center gap-2 font-mono ${className}`}>
      <span aria-hidden="true" className="text-primary-400">{reduceMotion ? '⠿' : FRAMES[frame]}</span>
      {children ?? <span className="sr-only">Загрузка</span>}
    </span>
  );
}
