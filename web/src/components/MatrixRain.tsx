import { useEffect, useRef, useState } from 'react';
import { SpaceInvader } from './SpaceInvader';
import type { InvaderPose } from './SpaceInvader';

const CHARS = 'アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲン0123456789ABCDEF';

interface MatrixRainProps {
  active: boolean;
  onWakeUp?: () => void;
}

export function MatrixRain({ active, onWakeUp }: MatrixRainProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const columnsRef = useRef<number[]>([]);
  const rafRef = useRef<number>(0);
  const [invaderPose, setInvaderPose] = useState<InvaderPose>('sleep');
  const [invaderSpeech, setInvaderSpeech] = useState<string | null>('Zzz...');
  const [waking, setWaking] = useState(false);
  const prefersReducedMotion = useRef(
    typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );

  useEffect(() => {
    const mql = window.matchMedia('(prefers-reduced-motion: reduce)');
    const handler = (e: MediaQueryListEvent) => { prefersReducedMotion.current = e.matches; };
    mql.addEventListener('change', handler);
    return () => mql.removeEventListener('change', handler);
  }, []);

  // Handle user activity — wake up sequence
  useEffect(() => {
    if (!active) return;

    const handleActivity = () => {
      if (waking) return;
      setWaking(true);
      setInvaderPose('teleport');
      setInvaderSpeech('// что происходит?!');

      setTimeout(() => {
        setInvaderPose('attack');
        setInvaderSpeech('// не пугай так!');

        setTimeout(() => {
          onWakeUp?.();
          setWaking(false);
          setInvaderPose('sleep');
          setInvaderSpeech('Zzz...');
        }, 1500);
      }, 1500);
    };

    window.addEventListener('mousemove', handleActivity, { once: true });
    window.addEventListener('keydown', handleActivity, { once: true });
    window.addEventListener('click', handleActivity, { once: true });

    return () => {
      window.removeEventListener('mousemove', handleActivity);
      window.removeEventListener('keydown', handleActivity);
      window.removeEventListener('click', handleActivity);
    };
  }, [active, waking, onWakeUp]);

  // Canvas matrix rain effect
  useEffect(() => {
    if (!active) return;
    if (prefersReducedMotion.current) return;

    const canvas = canvasRef.current;
    if (!canvas) return;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const resize = () => {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
      const cols = Math.floor(canvas.width / 16);
      columnsRef.current = Array(cols).fill(0).map(() => Math.random() * canvas.height);
    };

    resize();
    window.addEventListener('resize', resize);

    const draw = () => {
      if (prefersReducedMotion.current) {
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        return;
      }

      ctx.fillStyle = 'rgba(0, 0, 0, 0.05)';
      ctx.fillRect(0, 0, canvas.width, canvas.height);

      ctx.fillStyle = '#0f0';
      ctx.font = '14px monospace';

      columnsRef.current.forEach((y, i) => {
        const char = CHARS[Math.floor(Math.random() * CHARS.length)];
        const x = i * 16;

        // Random brightness
        if (Math.random() > 0.8) {
          ctx.fillStyle = '#4ade80';
        } else {
          ctx.fillStyle = '#22c55e80';
        }

        ctx.fillText(char, x, y);

        if (y > canvas.height && Math.random() > 0.975) {
          columnsRef.current[i] = 0;
        } else {
          columnsRef.current[i] = y + 16;
        }
      });

      rafRef.current = requestAnimationFrame(draw);
    };

    rafRef.current = requestAnimationFrame(draw);

    return () => {
      cancelAnimationFrame(rafRef.current);
      window.removeEventListener('resize', resize);
    };
  }, [active]);

  if (!active) return null;

  return (
    <div className="fixed inset-0 z-[90] pointer-events-auto" style={{ backgroundColor: 'rgba(0,0,0,0.85)' }}>
      <canvas ref={canvasRef} className="absolute inset-0" />
      <div className="absolute inset-0 flex items-center justify-center">
        <SpaceInvader
          size="lg"
          controlledPose={invaderPose}
          speechBubble={invaderSpeech}
          eyeOverride={invaderPose === 'sleep' ? 'closed' : undefined}
        />
      </div>
    </div>
  );
}
