import { useRef, useState, useEffect, useCallback, type ReactNode } from 'react';

// --- Types ---
export type InvaderPose = 'idle' | 'handsUp' | 'dance' | 'run' | 'spin'
  | 'cry' | 'sleep' | 'fly' | 'attack' | 'shield' | 'teleport' | 'transform';

export interface SpaceInvaderProps {
  size?: 'sm' | 'md' | 'lg';
  className?: string;
  interactive?: boolean;
  onPoseChange?: (pose: InvaderPose) => void;
  eyeOverride?: 'closed' | 'sad' | 'wide' | null;
  shake?: boolean;
  jump?: boolean;
  speechBubble?: string | null;
  colorFilter?: string;
  controlledPose?: InvaderPose | null;
}

// --- Colors (violet/purple) ---
const BODY_COLOR = '#8b5cf6';
const PUPIL_COLOR = '#ffffff';
const EYE_BG_COLOR = '#2e1065';

// --- Size map ---
const SIZE_MAP: Record<string, string> = { sm: '5px', md: '8px', lg: '12px' };

// --- Particle colors ---
const PARTICLE_COLORS = ['#8b5cf6', '#a78bfa', '#4ade80', '#ffffff', '#c4b5fd'];

// --- ASCII art helpers ---
function body(n: number, softEdges = false): string {
  const chars: string[] = [];
  for (let i = 0; i < n; i++) {
    chars.push(i % 3 === 2 ? '#' : '@');
  }
  if (softEdges && n >= 2) {
    chars[0] = '+';
    chars[n - 1] = chars[n - 1] === '#' ? '*' : '+';
  }
  return chars.join('');
}

const SP4 = '    ';

// --- Pre-compute all pose rows (avoid recomputation on render) ---
const IDLE_TOP = [
  SP4+SP4+'+**+'+SP4+SP4+SP4+SP4+SP4+'+**+'+SP4+SP4,
  SP4+SP4+SP4+'+**+'+SP4+SP4+SP4+'+**+'+SP4+SP4+SP4,
  SP4+SP4+body(28, true)+SP4+SP4,
  SP4+body(36, true)+SP4,
];
const IDLE_ARM_ROWS = [
  body(4, true)+SP4+body(28)+SP4+body(4, true),
  SP4+body(4, true)+body(28)+body(4, true)+SP4,
];
const IDLE_BODY_ROWS = [
  body(44, true),
  body(4, true)+SP4+body(28)+SP4+body(4, true),
  body(4, true)+SP4+body(4)+SP4+SP4+SP4+SP4+SP4+body(4)+SP4+body(4, true),
];
const IDLE_LEGS = [
  SP4+SP4+SP4+body(8, true)+SP4+body(8, true)+SP4+SP4+SP4,
];

const HANDSUP_TOP = [
  body(4, true)+SP4+SP4+SP4+SP4+SP4+SP4+SP4+SP4+SP4+body(4, true),
  SP4+body(4, true)+SP4+SP4+SP4+SP4+SP4+SP4+SP4+body(4, true)+SP4,
  SP4+SP4+body(4, true)+'+**+'+SP4+SP4+SP4+'+**+'+body(4, true)+SP4+SP4,
  SP4+SP4+SP4+'+**+'+SP4+SP4+SP4+'+**+'+SP4+SP4+SP4,
  SP4+SP4+body(28, true)+SP4+SP4,
  SP4+body(36, true)+SP4,
];
const HANDSUP_BODY_ROWS = [
  body(44, true),
  SP4+SP4+body(32, true)+SP4+SP4,
  SP4+SP4+body(4)+SP4+SP4+SP4+SP4+SP4+SP4+body(4)+SP4+SP4,
];
const HANDSUP_LEGS = IDLE_LEGS;

// --- Eye system ---
const EYE_PRE = [SP4 + body(8, true), SP4 + body(8, true), body(12, true)];
const EYE_MID = body(4);
const EYE_SUF = [body(8, true) + SP4, body(8, true) + SP4, body(12, true)];

interface EyePos { col: number; row: number; }

function getEyeLine(pos: EyePos, lineRow: number, wide = false): string {
  if (lineRow !== pos.row) return '........';
  if (wide) {
    const wCol = Math.max(0, Math.min(4, pos.col - 1));
    return '.'.repeat(wCol) + '@@@@' + '.'.repeat(4 - wCol);
  }
  return '.'.repeat(pos.col) + '@@' + '.'.repeat(6 - pos.col);
}

function getClosedEyeLine(lineRow: number): string {
  return lineRow === 1 ? '--------' : '........';
}

// --- Cached style objects (avoid recreating on each render) ---
const BODY_STYLE = { color: BODY_COLOR } as const;
const EYE_BG_STYLE = { color: EYE_BG_COLOR } as const;
const PUPIL_STYLE = {
  color: PUPIL_COLOR,
  textShadow: '0 0 6px rgba(255,255,255,0.8)',
} as const;
const CLOSED_EYE_STYLE = { color: BODY_COLOR, opacity: 0.7 } as const;

// --- Rendering helper ---
function renderEyeSegment(content: string): ReactNode[] {
  const parts: ReactNode[] = [];
  let i = 0;
  let key = 0;
  while (i < content.length) {
    if (content[i] === '@') {
      let j = i;
      while (j < content.length && content[j] === '@') j++;
      parts.push(<span key={key++} style={PUPIL_STYLE}>{content.slice(i, j)}</span>);
      i = j;
    } else if (content[i] === '-') {
      let j = i;
      while (j < content.length && content[j] === '-') j++;
      parts.push(<span key={key++} style={CLOSED_EYE_STYLE}>{content.slice(i, j)}</span>);
      i = j;
    } else {
      let j = i;
      while (j < content.length && content[j] !== '@' && content[j] !== '-') j++;
      parts.push(<span key={key++} style={EYE_BG_STYLE}>{content.slice(i, j)}</span>);
      i = j;
    }
  }
  return parts;
}

// --- Particle ---
interface Particle {
  id: number;
  vx: number;
  vy: number;
  color: string;
  size: number;
}

// --- Main component ---
export function SpaceInvader({
  size = 'md',
  className = '',
  interactive = false,
  onPoseChange,
  eyeOverride = null,
  shake = false,
  jump = false,
  speechBubble = null,
  colorFilter,
  controlledPose = null,
}: SpaceInvaderProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [eyePos, setEyePos] = useState<EyePos>({ col: 3, row: 1 });
  const [pose, setPose] = useState<InvaderPose>('idle');
  const [particles, setParticles] = useState<Particle[]>([]);
  const [animClass, setAnimClass] = useState('');

  // Use refs for state that only affects eye rendering to avoid re-render cascades
  const isHoveredRef = useRef(false);
  const cursorGoneRef = useRef(false);
  const [, forceEyeUpdate] = useState(0);

  const rafRef = useRef(0);
  const lastRef = useRef('3-1');
  const clickCountRef = useRef(0);
  const clickTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const poseTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const idleTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const particleIdRef = useRef(0);

  // Sync controlled pose from parent
  useEffect(() => {
    if (controlledPose) setPose(controlledPose);
  }, [controlledPose]);

  // Pose change callback
  useEffect(() => {
    onPoseChange?.(pose);
  }, [pose, onPoseChange]);

  // External shake trigger
  useEffect(() => {
    if (shake) {
      setAnimClass('animate-shake');
      const t = setTimeout(() => setAnimClass(''), 500);
      return () => clearTimeout(t);
    }
  }, [shake]);

  // External jump trigger
  useEffect(() => {
    if (jump) {
      setAnimClass('animate-bounce-in');
      const t = setTimeout(() => setAnimClass(''), 600);
      return () => clearTimeout(t);
    }
  }, [jump]);

  // --- Eye tracking (RAF-throttled) ---
  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      if (rafRef.current) return;
      rafRef.current = requestAnimationFrame(() => {
        rafRef.current = 0;
        if (!containerRef.current) return;

        const rect = containerRef.current.getBoundingClientRect();
        const cx = rect.left + rect.width / 2;
        const cy = rect.top + rect.height / 2;
        const dx = e.clientX - cx;
        const dy = e.clientY - cy;
        const dist = Math.sqrt(dx * dx + dy * dy);

        let col = 3, row = 1;
        if (dist > 50) {
          const angle = Math.atan2(dy, dx);
          const intensity = Math.min(1, dist / 250);
          col = Math.round(3 + Math.cos(angle) * 3 * intensity);
          row = Math.round(1 + Math.sin(angle) * 1 * intensity);
          col = Math.max(0, Math.min(6, col));
          row = Math.max(0, Math.min(2, row));
        }

        const key = `${col}-${row}`;
        if (key !== lastRef.current) {
          lastRef.current = key;
          setEyePos({ col, row });
        }
      });
    };

    window.addEventListener('mousemove', onMouseMove, { passive: true });
    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      if (rafRef.current) cancelAnimationFrame(rafRef.current);
    };
  }, []);

  // --- Cursor leave/enter detection (stable — no state in deps) ---
  useEffect(() => {
    if (!interactive) return;

    const onLeave = () => {
      cursorGoneRef.current = true;
      forceEyeUpdate(c => c + 1);
      poseTimerRef.current = setTimeout(() => {
        setPose('handsUp');
      }, 3000);
    };
    const onEnter = () => {
      if (cursorGoneRef.current) {
        cursorGoneRef.current = false;
        clearTimeout(poseTimerRef.current);
        setPose('idle');
        setAnimClass('animate-bounce-in');
        spawnParticles(6);
        setTimeout(() => setAnimClass(''), 600);
        forceEyeUpdate(c => c + 1);
      }
    };

    document.addEventListener('mouseleave', onLeave);
    document.addEventListener('mouseenter', onEnter);
    return () => {
      document.removeEventListener('mouseleave', onLeave);
      document.removeEventListener('mouseenter', onEnter);
      clearTimeout(poseTimerRef.current);
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [interactive]);

  // --- Idle boredom timer (debounced — only resets every 2s max) ---
  useEffect(() => {
    if (!interactive) return;

    let lastReset = 0;

    const resetIdle = () => {
      const now = Date.now();
      if (now - lastReset < 2000) return; // debounce: max 1 reset per 2s
      lastReset = now;
      clearTimeout(idleTimerRef.current);
      idleTimerRef.current = setTimeout(() => {
        const actions: InvaderPose[] = ['dance', 'handsUp'];
        setPose(actions[Math.floor(Math.random() * actions.length)]);
        setTimeout(() => setPose('idle'), 2500);
      }, 10000);
    };

    resetIdle();
    const onActivity = () => resetIdle();
    window.addEventListener('mousemove', onActivity, { passive: true });
    window.addEventListener('click', onActivity, { passive: true });

    return () => {
      clearTimeout(idleTimerRef.current);
      window.removeEventListener('mousemove', onActivity);
      window.removeEventListener('click', onActivity);
    };
  }, [interactive]);

  // --- Particle spawner ---
  const spawnParticles = useCallback((count: number) => {
    const ids: number[] = [];
    const newParticles: Particle[] = [];
    for (let i = 0; i < count; i++) {
      const id = particleIdRef.current++;
      ids.push(id);
      newParticles.push({
        id,
        vx: (Math.random() - 0.5) * 100,
        vy: -(Math.random() * 60 + 20),
        color: PARTICLE_COLORS[Math.floor(Math.random() * PARTICLE_COLORS.length)],
        size: Math.random() * 3 + 2,
      });
    }
    setParticles(prev => [...prev, ...newParticles]);
    setTimeout(() => {
      setParticles(prev => prev.filter(p => !ids.includes(p.id)));
    }, 700);
  }, []);

  // --- Click handler ---
  const handleClick = useCallback(() => {
    if (!interactive) return;

    clickCountRef.current++;
    const count = clickCountRef.current;

    clearTimeout(clickTimerRef.current);
    clickTimerRef.current = setTimeout(() => { clickCountRef.current = 0; }, 2000);

    if (count >= 5) {
      clickCountRef.current = 0;
      setPose('run');
      spawnParticles(10);
      setTimeout(() => setPose('idle'), 3000);
      return;
    }

    setAnimClass('');
    requestAnimationFrame(() => {
      setAnimClass('animate-bounce-in');
      spawnParticles(Math.min(6 + count * 2, 12));
    });
    setTimeout(() => setAnimClass(''), 600);
  }, [interactive, spawnParticles]);

  // --- Double click ---
  const handleDoubleClick = useCallback(() => {
    if (!interactive) return;
    setPose('handsUp');
    spawnParticles(8);
    setTimeout(() => { setPose('dance'); }, 500);
    setTimeout(() => setPose('idle'), 2500);
  }, [interactive, spawnParticles]);

  // --- Long press ---
  const handlePointerDown = useCallback(() => {
    if (!interactive) return;
    longPressTimerRef.current = setTimeout(() => { setPose('spin'); }, 500);
  }, [interactive]);

  const handlePointerUp = useCallback(() => {
    clearTimeout(longPressTimerRef.current);
    if (pose === 'spin') {
      setTimeout(() => setPose('idle'), 300);
    }
  }, [pose]);

  // --- Hover (use ref + minimal re-render) ---
  const handleMouseEnter = useCallback(() => {
    if (!interactive) return;
    isHoveredRef.current = true;
    forceEyeUpdate(c => c + 1);
  }, [interactive]);

  const handleMouseLeave = useCallback(() => {
    isHoveredRef.current = false;
    forceEyeUpdate(c => c + 1);
  }, []);

  // --- Determine effective eye state ---
  const eyeState = (() => {
    if (eyeOverride === 'closed') return 'closed';
    if (eyeOverride === 'sad') return 'sad';
    if (eyeOverride === 'wide') return 'wide';
    if (cursorGoneRef.current) return 'sad';
    if (isHoveredRef.current) return 'wide';
    return 'normal';
  })();

  // --- Build rows ---
  const bodyLine = (text: string, key: string) => (
    <div key={key} style={BODY_STYLE}>{text}</div>
  );

  const eyeRow = (lineRow: number, key: string) => {
    let leftEye: string;
    let rightEye: string;

    if (eyeState === 'closed') {
      leftEye = getClosedEyeLine(lineRow);
      rightEye = getClosedEyeLine(lineRow);
    } else if (eyeState === 'sad') {
      const sadPos = { col: 3, row: 2 };
      leftEye = getEyeLine(sadPos, lineRow);
      rightEye = getEyeLine(sadPos, lineRow);
    } else if (eyeState === 'wide') {
      leftEye = getEyeLine(eyePos, lineRow, true);
      rightEye = getEyeLine(eyePos, lineRow, true);
    } else {
      leftEye = getEyeLine(eyePos, lineRow);
      rightEye = getEyeLine(eyePos, lineRow);
    }

    return (
      <div key={key}>
        <span style={BODY_STYLE}>{EYE_PRE[lineRow]}</span>
        {renderEyeSegment(leftEye)}
        <span style={BODY_STYLE}>{EYE_MID}</span>
        {renderEyeSegment(rightEye)}
        <span style={BODY_STYLE}>{EYE_SUF[lineRow]}</span>
      </div>
    );
  };

  const renderIdlePose = () => (
    <>
      {IDLE_TOP.flatMap((line, i) => [bodyLine(line, `t${i}a`), bodyLine(line, `t${i}b`)])}
      {[0, 1, 2].flatMap((r) => [eyeRow(r, `e${r}a`), eyeRow(r, `e${r}b`)])}
      {IDLE_ARM_ROWS.flatMap((line, i) => [bodyLine(line, `a${i}a`), bodyLine(line, `a${i}b`)])}
      {IDLE_BODY_ROWS.flatMap((line, i) => [bodyLine(line, `m${i}a`), bodyLine(line, `m${i}b`)])}
      <div style={{ animation: 'tentacle-wiggle 2s ease-in-out infinite' }}>
        {IDLE_LEGS.flatMap((line, i) => [bodyLine(line, `l${i}a`), bodyLine(line, `l${i}b`)])}
      </div>
    </>
  );

  const renderHandsUpPose = () => (
    <>
      {HANDSUP_TOP.flatMap((line, i) => [bodyLine(line, `ht${i}a`), bodyLine(line, `ht${i}b`)])}
      {[0, 1, 2].flatMap((r) => [eyeRow(r, `he${r}a`), eyeRow(r, `he${r}b`)])}
      {HANDSUP_BODY_ROWS.flatMap((line, i) => [bodyLine(line, `hm${i}a`), bodyLine(line, `hm${i}b`)])}
      {HANDSUP_LEGS.flatMap((line, i) => [bodyLine(line, `hl${i}a`), bodyLine(line, `hl${i}b`)])}
    </>
  );

  const renderDancePose = () => (
    <div style={{ animation: 'wave-hands 0.4s ease-in-out infinite' }}>
      {renderHandsUpPose()}
    </div>
  );

  const renderRunPose = () => (
    <>
      {IDLE_TOP.flatMap((line, i) => [bodyLine(line, `rt${i}a`), bodyLine(line, `rt${i}b`)])}
      {[0, 1, 2].flatMap((r) => [eyeRow(r, `re${r}a`), eyeRow(r, `re${r}b`)])}
      {IDLE_ARM_ROWS.flatMap((line, i) => [bodyLine(line, `ra${i}a`), bodyLine(line, `ra${i}b`)])}
      {IDLE_BODY_ROWS.flatMap((line, i) => [bodyLine(line, `rm${i}a`), bodyLine(line, `rm${i}b`)])}
      <div style={{ animation: 'run-legs 0.2s ease-in-out infinite' }}>
        {IDLE_LEGS.flatMap((line, i) => [bodyLine(line, `rl${i}a`), bodyLine(line, `rl${i}b`)])}
      </div>
    </>
  );

  const renderCryPose = () => (
    <div style={{ position: 'relative' }}>
      {renderIdlePose()}
      {/* Tear particles */}
      {[0, 1].map((i) => (
        <div
          key={`tear-${i}`}
          style={{
            position: 'absolute',
            top: '40%',
            left: i === 0 ? '30%' : '62%',
            width: '3px',
            height: '6px',
            background: '#60a5fa',
            borderRadius: '50%',
            animation: 'tear-fall 1s ease-in infinite',
            animationDelay: `${i * 0.4}s`,
          }}
        />
      ))}
    </div>
  );

  const renderSleepPose = () => (
    <div style={{ position: 'relative' }}>
      {renderIdlePose()}
      {/* Zzz floating */}
      {['Z', 'z', 'z'].map((ch, i) => (
        <span
          key={`zzz-${i}`}
          style={{
            position: 'absolute',
            top: `${10 + i * 12}%`,
            right: `${5 - i * 8}%`,
            fontSize: `${14 - i * 2}px`,
            color: '#a78bfa',
            fontFamily: 'monospace',
            fontWeight: 'bold',
            opacity: 0.8,
            animation: 'zzz-float 2s ease-out infinite',
            animationDelay: `${i * 0.5}s`,
          }}
        >
          {ch}
        </span>
      ))}
    </div>
  );

  const renderFlyPose = () => (
    <div style={{ animation: 'fly-drift 2s ease-in-out infinite', position: 'relative' }}>
      {renderHandsUpPose()}
      {/* Rocket particles (downward) */}
      {[0, 1, 2].map((i) => (
        <div
          key={`flame-${i}`}
          style={{
            position: 'absolute',
            bottom: '-8px',
            left: `${35 + i * 12}%`,
            width: '4px',
            height: '10px',
            background: i === 1 ? '#f59e0b' : '#ef4444',
            borderRadius: '0 0 2px 2px',
            opacity: 0.8,
            animation: 'particle-fly 0.5s ease-out infinite',
            animationDelay: `${i * 0.15}s`,
            '--px': '0px',
            '--py': '15px',
          } as React.CSSProperties}
        />
      ))}
    </div>
  );

  const renderAttackPose = () => (
    <div style={{ position: 'relative' }}>
      {renderIdlePose()}
      {/* Flash effect */}
      <div
        style={{
          position: 'absolute',
          right: '-20px',
          top: '45%',
          color: '#ef4444',
          fontFamily: 'monospace',
          fontSize: '10px',
          fontWeight: 'bold',
          animation: 'fade-in 0.2s ease-out',
          whiteSpace: 'pre',
        }}
      >
        {'>>>--->'}
      </div>
    </div>
  );

  const renderShieldPose = () => (
    <div style={{ animation: 'shield-pulse 1.5s ease-in-out infinite', position: 'relative' }}>
      {renderIdlePose()}
      {/* Shield barrier overlay */}
      <div
        style={{
          position: 'absolute',
          inset: '-6px',
          border: '2px solid rgba(74,222,128,0.5)',
          borderRadius: '8px',
          pointerEvents: 'none',
        }}
      />
    </div>
  );

  const renderTeleportPose = () => (
    <div style={{ animation: 'teleport-glitch 0.8s ease-in-out' }}>
      {renderIdlePose()}
    </div>
  );

  const renderTransformPose = () => (
    <div style={{ filter: colorFilter || undefined }}>
      {renderIdlePose()}
    </div>
  );

  const renderPose = () => {
    switch (pose) {
      case 'handsUp': return renderHandsUpPose();
      case 'dance': return renderDancePose();
      case 'run': return renderRunPose();
      case 'cry': return renderCryPose();
      case 'sleep': return renderSleepPose();
      case 'fly': return renderFlyPose();
      case 'attack': return renderAttackPose();
      case 'shield': return renderShieldPose();
      case 'teleport': return renderTeleportPose();
      case 'transform': return renderTransformPose();
      case 'spin':
      case 'idle':
      default: return renderIdlePose();
    }
  };

  return (
    <div
      ref={containerRef}
      className={`${className} ${animClass}`}
      role="img"
      aria-label="Интерактивный space invader"
      style={{ display: 'inline-block', position: 'relative' }}
      onClick={handleClick}
      onDoubleClick={handleDoubleClick}
      onPointerDown={handlePointerDown}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerUp}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      {/* Speech bubble */}
      {speechBubble && (
        <div
          style={{
            position: 'absolute',
            top: '-2em',
            left: '50%',
            transform: 'translateX(-50%)',
            whiteSpace: 'nowrap',
            background: 'rgba(17,24,39,0.92)',
            color: '#a78bfa',
            fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
            fontSize: '11px',
            padding: '4px 10px',
            borderRadius: '6px',
            border: '1px solid rgba(139,92,246,0.3)',
            pointerEvents: 'none',
            zIndex: 30,
            animation: 'fade-in 0.2s ease-out',
          }}
        >
          {speechBubble}
          <div
            style={{
              position: 'absolute',
              bottom: '-4px',
              left: '50%',
              transform: 'translateX(-50%) rotate(45deg)',
              width: '8px',
              height: '8px',
              background: 'rgba(17,24,39,0.92)',
              borderRight: '1px solid rgba(139,92,246,0.3)',
              borderBottom: '1px solid rgba(139,92,246,0.3)',
            }}
          />
        </div>
      )}

      {/* Particles */}
      {particles.map((p) => (
        <div
          key={p.id}
          style={{
            position: 'absolute',
            left: '50%',
            top: '50%',
            width: p.size,
            height: p.size,
            backgroundColor: p.color,
            borderRadius: '1px',
            pointerEvents: 'none',
            zIndex: 20,
            '--px': `${p.vx}px`,
            '--py': `${p.vy}px`,
            animation: 'particle-fly 0.7s ease-out forwards',
          } as React.CSSProperties}
        />
      ))}

      {/* Single float animation container — glow via filter on container, NOT text-shadow per char */}
      <div style={{
        animation: pose === 'spin'
          ? 'spin-invader 0.5s linear infinite'
          : 'invader-float 3s ease-in-out infinite',
        willChange: 'transform',
      }}>
        <div style={{
          animation: 'invader-glow 2.5s ease-in-out infinite',
          willChange: 'filter',
        }}>
          <div
            style={{
              fontFamily: "'Courier New', Consolas, 'Liberation Mono', monospace",
              fontSize: SIZE_MAP[size] || SIZE_MAP.md,
              lineHeight: 1,
              whiteSpace: 'pre',
              userSelect: 'none',
              cursor: interactive ? 'pointer' : 'default',
            }}
          >
            {renderPose()}
          </div>
        </div>
      </div>
    </div>
  );
}
