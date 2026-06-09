import { useState, useEffect, type ReactNode } from 'react';
import type { InvaderPose } from '../SpaceInvader';

interface QuestEnvironmentProps {
  level: number;
  invaderPose: InvaderPose;
  children: ReactNode;
}

const MONO: React.CSSProperties = {
  fontFamily: "'Courier New', Consolas, 'Liberation Mono', monospace",
  fontSize: '10px',
  lineHeight: 1.4,
  whiteSpace: 'pre',
  userSelect: 'none',
  pointerEvents: 'none',
};

const SIDE_ROWS = 18;

export function QuestEnvironment({ level, invaderPose, children }: QuestEnvironmentProps) {
  const [attackGlitch, setAttackGlitch] = useState(false);

  useEffect(() => {
    if (invaderPose !== 'attack') return;
    // setState через макротаску, а не синхронно в эффекте: иначе каскадный
    // ререндер (react-hooks/set-state-in-effect).
    const on = setTimeout(() => setAttackGlitch(true), 0);
    const off = setTimeout(() => setAttackGlitch(false), 500);
    return () => {
      clearTimeout(on);
      clearTimeout(off);
    };
  }, [invaderPose]);

  if (level === 0) return <>{children}</>;

  const guardAsleep = invaderPose === 'sleep';

  return (
    <div style={{ position: 'relative', overflow: 'visible' }}>
      <EnvironmentFrame level={level} attackGlitch={attackGlitch} guardAsleep={guardAsleep}>
        {children}
      </EnvironmentFrame>
    </div>
  );
}

function EnvironmentFrame({
  level, attackGlitch, guardAsleep, children,
}: {
  level: number; attackGlitch: boolean; guardAsleep: boolean; children: ReactNode;
}) {
  const top = useTopFrame(level, attackGlitch);
  const bottom = useBottomFrame(level);
  const { left, right } = useSideFrames(level, guardAsleep);
  const cls = attackGlitch ? 'animate-env-attack-glitch' : '';

  return (
    <div className={cls} style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', overflow: 'visible' }}>
      {top}
      <div style={{ display: 'flex', alignItems: 'stretch', overflow: 'visible' }}>
        {left}
        <div style={{ padding: '1.5em 2.5em 1em', overflow: 'visible' }}>{children}</div>
        {right}
      </div>
      {bottom}
    </div>
  );
}

// ── Top frames ────────────────────────────────────────────────────
function useTopFrame(level: number, attackGlitch: boolean) {
  const [ledOn, setLedOn] = useState(true);
  const [fireFrame, setFireFrame] = useState(0);
  const [glitchSeed, setGlitchSeed] = useState(0);
  const [camOn, setCamOn] = useState(true);
  const [twinkle, setTwinkle] = useState(0);

  useEffect(() => {
    if (level === 1 || level === 4) {
      const id = setInterval(() => { setLedOn(v => !v); setCamOn(v => !v); }, 800);
      return () => clearInterval(id);
    }
    if (level === 2) {
      const id = setInterval(() => setFireFrame(f => (f + 1) % 3), 400);
      return () => clearInterval(id);
    }
    if (level === 3) {
      const id = setInterval(() => setGlitchSeed(s => s + 1), 300);
      return () => clearInterval(id);
    }
    if (level === 5) {
      const id = setInterval(() => setTwinkle(t => t + 1), 600);
      return () => clearInterval(id);
    }
  }, [level]);

  const corrupt = (ch: string, idx: number) => {
    const thresh = attackGlitch ? 0.3 : 0.7;
    const r = (Math.sin(glitchSeed * 17 + idx * 31) * 10000) % 1;
    const rr = r < 0 ? r + 1 : r;
    if (rr > thresh) {
      const reps = ['~', '#', '%', '?', '·', '¦'];
      return <span key={idx} style={{ color: 'rgb(239,68,68)' }}>{reps[Math.floor(rr * reps.length)]}</span>;
    }
    return ch;
  };

  switch (level) {
    case 1: {
      const led = ledOn ? '●' : '○';
      const c = 'rgb(168,85,247)';
      const lc = ledOn ? 'rgb(192,132,252)' : 'rgb(107,33,168)';
      const g = '0 0 6px rgba(139,92,246,0.5)';
      return (
        <pre style={{ ...MONO, color: c, textShadow: g, textAlign: 'center' }}>
          {`╔══╦══╦══╦══╦══╦══╦══╦══╦══╦══╗\n`}
          <span style={{ color: lc }}>{led}</span>{`      [LOCKED]      `}<span style={{ color: lc }}>{led}</span>{`\n`}
          {`╠══╬══╬══╬══╬══╬══╬══╬══╬══╬══╣`}
        </pre>
      );
    }
    case 2: {
      const p = [['▓','░','▒'],['▒','▓','░'],['░','▒','▓']][fireFrame];
      return (
        <pre style={{ ...MONO, color: 'rgb(251,146,60)', textShadow: '0 0 8px rgba(239,68,68,0.5)', textAlign: 'center' }}>
          {`${p[0]}${p[1]}${p[2]}  `}<span style={{ color: 'rgb(239,68,68)' }}>FIREWALL v3.1</span>{`  ${p[2]}${p[1]}${p[0]}\n`}
          {`${p[2]}${p[0]}══════════════════════${p[0]}${p[2]}`}
        </pre>
      );
    }
    case 3: {
      const line = '╔═══════ERR0R_Z0NE═══════╗';
      return (
        <pre style={{ ...MONO, color: 'rgb(234,179,8)', textShadow: '0 0 4px rgba(234,179,8,0.6)', textAlign: 'center' }}>
          {line.split('').map((ch, i) => {
            if ((ch >= 'A' && ch <= 'z') || ch === '_' || ch === '0' || ch === ' ') return ch;
            return corrupt(ch, i);
          })}
        </pre>
      );
    }
    case 4: {
      const cam = camOn ? '[CAM]' : '[   ]';
      const cc = camOn ? 'rgb(34,211,238)' : 'rgb(30,58,138)';
      return (
        <pre style={{ ...MONO, color: 'rgb(59,130,246)', textShadow: '0 0 6px rgba(59,130,246,0.5)', textAlign: 'center' }}>
          {`▓▓▓`}<span style={{ color: cc }}>{cam}</span>{`▓▓▓▓▓▓▓▓▓`}<span style={{ color: cc }}>{cam}</span>{`▓▓▓\n`}
          {`▓▓══════════════════════▓▓`}
        </pre>
      );
    }
    case 5: {
      const sop = (i: number) => ((twinkle + i * 3) % 5 < 2 ? 0.3 : 1);
      const cloud = 'rgb(156,163,175)';
      return (
        <pre style={{ ...MONO, color: 'rgb(107,114,128)', textAlign: 'center' }}>
          {' '}<span style={{ color: '#fff', opacity: sop(0) }}>*</span>{'    · '}<span style={{ color: '#fff', opacity: sop(1) }}>*</span>{'    ☆    '}<span style={{ color: '#fff', opacity: sop(2) }}>*</span>{'   ·\n'}
          {'· '}<span style={{ color: cloud }}>{'~~~☁~~~'}</span>{'  ·  '}<span style={{ color: cloud }}>{'~~~☁~~~'}</span>{' ·'}
        </pre>
      );
    }
    default: return null;
  }
}

// ── Bottom frames ─────────────────────────────────────────────────
function useBottomFrame(level: number) {
  const [ledOn, setLedOn] = useState(true);
  const [glitchSeed, setGlitchSeed] = useState(0);

  useEffect(() => {
    if (level === 1) {
      const id = setInterval(() => setLedOn(v => !v), 800);
      return () => clearInterval(id);
    }
    if (level === 3) {
      const id = setInterval(() => setGlitchSeed(s => s + 1), 300);
      return () => clearInterval(id);
    }
  }, [level]);

  const corrupt = (ch: string, idx: number) => {
    const r = (Math.sin(glitchSeed * 17 + idx * 31) * 10000) % 1;
    const rr = r < 0 ? r + 1 : r;
    if (rr > 0.7) {
      const reps = ['~', '#', '%', '?', '·'];
      return <span key={idx} style={{ color: 'rgb(239,68,68)' }}>{reps[Math.floor(rr * reps.length)]}</span>;
    }
    return ch;
  };

  switch (level) {
    case 1: {
      const led = ledOn ? '●' : '○';
      const c = 'rgb(168,85,247)';
      const lc = ledOn ? 'rgb(192,132,252)' : 'rgb(107,33,168)';
      const g = '0 0 6px rgba(139,92,246,0.5)';
      return (
        <pre style={{ ...MONO, color: c, textShadow: g, textAlign: 'center' }}>
          {`╠══╬══╬══╬══╬══╬══╬══╬══╬══╬══╣\n`}
          <span style={{ color: lc }}>{led}</span>{`      root@srv      `}<span style={{ color: lc }}>{led}</span>{`\n`}
          {`╚══╩══╩══╩══╩══╩══╩══╩══╩══╩══╝`}
        </pre>
      );
    }
    case 2:
      return (
        <pre style={{ ...MONO, color: 'rgb(251,146,60)', textShadow: '0 0 8px rgba(239,68,68,0.5)', textAlign: 'center' }}>
          {`▓░══════════════════════░▓\n`}
          <span style={{ color: 'rgb(239,68,68)' }}>{'▓░▒ ACCESS DENIED ▒░▓'}</span>
        </pre>
      );
    case 3: {
      const line = '╚═══════════════════════════╝';
      return (
        <pre style={{ ...MONO, color: 'rgb(234,179,8)', textShadow: '0 0 4px rgba(234,179,8,0.6)' }}>
          {line.split('').map((ch, i) => corrupt(ch, i + 500))}
        </pre>
      );
    }
    case 4:
      return (
        <pre style={{ ...MONO, color: 'rgb(59,130,246)', textShadow: '0 0 6px rgba(59,130,246,0.5)', textAlign: 'center' }}>
          {`▓▓══════════════════════▓▓\n`}
          {`▓▓▓ `}<span style={{ color: 'rgb(34,211,238)' }}>SECURITY ZONE</span>{` ▓▓▓`}
        </pre>
      );
    case 5:
      return (
        <pre style={{ ...MONO, color: 'rgb(74,222,128)', textShadow: '0 0 8px rgba(74,222,128,0.5)', textAlign: 'center' }}>
          {'·       ·       ·\n'}
          <span style={{ fontWeight: 'bold' }}>{'▲  FREEDOM  ▲'}</span>
        </pre>
      );
    default: return null;
  }
}

// ── Side frames ───────────────────────────────────────────────────
function useSideFrames(level: number, guardAsleep: boolean) {
  const mk = (ch: string) => Array(SIDE_ROWS).fill(ch).join('\n');

  switch (level) {
    case 1: {
      const c = 'rgb(168,85,247)';
      const g = '0 0 6px rgba(139,92,246,0.5)';
      return {
        left: <pre style={{ ...MONO, color: c, textShadow: g }}>{mk('║  ║')}</pre>,
        right: <pre style={{ ...MONO, color: c, textShadow: g }}>{mk('║  ║')}</pre>,
      };
    }
    case 2: {
      const c = 'rgb(251,146,60)';
      const g = '0 0 8px rgba(239,68,68,0.5)';
      return {
        left: <pre style={{ ...MONO, color: c, textShadow: g }}>{mk('▓░▒░')}</pre>,
        right: <pre style={{ ...MONO, color: c, textShadow: g }}>{mk('░▒░▓')}</pre>,
      };
    }
    case 3: {
      const c = 'rgb(234,179,8)';
      const g = '0 0 4px rgba(234,179,8,0.6)';
      return {
        left: <pre style={{ ...MONO, color: c, textShadow: g }}>{mk('║')}</pre>,
        right: <pre style={{ ...MONO, color: c, textShadow: g }}>{mk('║')}</pre>,
      };
    }
    case 4: {
      const c = 'rgb(59,130,246)';
      const g = '0 0 6px rgba(59,130,246,0.5)';
      const ga = guardAsleep ? '(-.-)zzZ' : '(o_o)!  ';
      const gc = guardAsleep ? 'rgb(74,222,128)' : 'rgb(239,68,68)';
      const right = Array.from({ length: SIDE_ROWS }, (_, i) => {
        if (i === Math.floor(SIDE_ROWS / 2)) return <span key={i}><span style={{ color: gc }}>{ga}</span></span>;
        return '▓▓';
      });
      return {
        left: <pre style={{ ...MONO, color: c, textShadow: g }}>{mk('▓▓')}</pre>,
        right: (
          <pre style={{ ...MONO, color: c, textShadow: g }}>
            {right.map((l, i) => <span key={i}>{l}{i < SIDE_ROWS - 1 ? '\n' : ''}</span>)}
          </pre>
        ),
      };
    }
    case 5: {
      const c = 'rgb(107,114,128)';
      return {
        left: <pre style={{ ...MONO, color: c }}>{mk('  ·')}</pre>,
        right: <pre style={{ ...MONO, color: c }}>{mk('·  ')}</pre>,
      };
    }
    default:
      return { left: null, right: null };
  }
}
