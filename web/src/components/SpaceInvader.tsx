import { useRef, useState, useEffect, type ReactNode } from 'react';

interface SpaceInvaderProps {
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

// --- Colors ---
const BODY_COLOR = '#c084fc';   // purple-400
const PUPIL_COLOR = '#ffffff';
const EYE_BG_COLOR = '#3b1a6e'; // visible dark purple for eye socket on both themes

// --- Size map ---
const SIZE_MAP: Record<string, string> = { sm: '5px', md: '8px', lg: '12px' };

// --- Classic "crab" space invader (👾) ---
// 11-pixel grid, 4 chars/pixel = 44 chars wide.
// Each row doubled (2 lines per pixel row) for correct aspect ratio.
// Total: 22 lines × 44 chars — nearly square proportions.
// Uses @@# texture for body, +**+ for antennae, dots for eyes.

// Helper: generate textured body string of given length
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

// TOP rows (each will be rendered twice)
const TOP = [
  SP4+SP4+'+**+'+SP4+SP4+SP4+SP4+SP4+'+**+'+SP4+SP4,       // antennae (px 2,8)
  SP4+SP4+SP4+'+**+'+SP4+SP4+SP4+'+**+'+SP4+SP4+SP4,        // stems (px 3,7)
  SP4+SP4+body(28, true)+SP4+SP4,                             // head top (px 2-8)
  SP4+body(36, true)+SP4,                                      // head fill (px 1-9)
];

// Eye rows: prefix(12) + leftEye(8) + mid(4) + rightEye(8) + suffix(12) = 44
const EYE_PRE = [
  SP4 + body(8, true),    // rows 0,1: .XX body
  SP4 + body(8, true),
  body(12, true),          // row 2: XXX body
];
const EYE_MID = body(4);
const EYE_SUF = [
  body(8, true) + SP4,    // rows 0,1: XX. body
  body(8, true) + SP4,
  body(12, true),          // row 2: XXX body
];

// Arm rows between eyes and body
const ARM_ROWS = [
  body(4, true)+SP4+body(28)+SP4+body(4, true),               // arms out
  SP4+body(4, true)+body(28)+body(4, true)+SP4,               // arms angle in
];

// Body rows below arms
const BODY_ROWS = [
  body(44, true),                                              // full body (all 11 px)
  body(4, true)+SP4+body(28)+SP4+body(4, true),               // cutouts (px 0,2-8,10)
  body(4, true)+SP4+body(4)+SP4+SP4+SP4+SP4+SP4+body(4)+SP4+body(4, true), // legs outer
];

// Leg rows
const LEGS = [
  SP4+SP4+SP4+body(8, true)+SP4+body(8, true)+SP4+SP4+SP4,   // legs inner (px 3-4, 6-7)
];

// --- Eye system: 7x3 grid (21 positions) ---
// Eye socket is 8 chars wide × 3 rows tall.
// Pupil '@@' (2 chars) can be at col 0-6, row 0-2.

interface EyePos {
  col: number; // 0-6
  row: number; // 0-2
}

function getEyeLine(pos: EyePos, lineRow: number): string {
  const bg = '........'; // 8 dots
  if (lineRow !== pos.row) return bg;
  return '.'.repeat(pos.col) + '@@' + '.'.repeat(6 - pos.col);
}

// --- Rendering helper: color eye segment chars ---
function renderEyeSegment(content: string): ReactNode[] {
  const parts: ReactNode[] = [];
  let i = 0;
  let key = 0;
  while (i < content.length) {
    if (content[i] === '@') {
      let j = i;
      while (j < content.length && content[j] === '@') j++;
      parts.push(
        <span
          key={key++}
          style={{
            color: PUPIL_COLOR,
            textShadow: '0 0 8px rgba(255,255,255,0.9), 0 0 16px rgba(192,132,252,0.6)',
          }}
        >
          {content.slice(i, j)}
        </span>,
      );
      i = j;
    } else {
      let j = i;
      while (j < content.length && content[j] !== '@') j++;
      parts.push(
        <span key={key++} style={{ color: EYE_BG_COLOR }}>
          {content.slice(i, j)}
        </span>,
      );
      i = j;
    }
  }
  return parts;
}

// --- Main component ---

export function SpaceInvader({ size = 'md', className = '' }: SpaceInvaderProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [eyePos, setEyePos] = useState<EyePos>({ col: 3, row: 1 });
  const rafRef = useRef(0);
  const lastRef = useRef('3-1');

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

        let col = 3, row = 1; // center
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

  // --- Build rows (each pixel row rendered twice for correct aspect ratio) ---
  const bodyLine = (text: string, key: string) => (
    <div key={key} style={{ color: BODY_COLOR }}>{text}</div>
  );

  const eyeRow = (lineRow: number, key: string) => {
    const leftEye = getEyeLine(eyePos, lineRow);
    const rightEye = getEyeLine(eyePos, lineRow);
    return (
      <div key={key}>
        <span style={{ color: BODY_COLOR }}>{EYE_PRE[lineRow]}</span>
        {renderEyeSegment(leftEye)}
        <span style={{ color: BODY_COLOR }}>{EYE_MID}</span>
        {renderEyeSegment(rightEye)}
        <span style={{ color: BODY_COLOR }}>{EYE_SUF[lineRow]}</span>
      </div>
    );
  };

  return (
    <div
      ref={containerRef}
      className={className}
      role="img"
      aria-label="Интерактивный space invader"
      style={{ display: 'inline-block' }}
    >
      <div style={{ animation: 'invader-float 3s ease-in-out infinite' }}>
        <div style={{ animation: 'invader-breathe 4s ease-in-out infinite' }}>
          <div
            style={{
              fontFamily: "'Courier New', Consolas, 'Liberation Mono', monospace",
              fontSize: SIZE_MAP[size] || SIZE_MAP.md,
              lineHeight: 1,
              whiteSpace: 'pre',
              animation: 'invader-glow 2.5s ease-in-out infinite',
              userSelect: 'none',
            }}
          >
            {TOP.flatMap((line, i) => [bodyLine(line, `t${i}a`), bodyLine(line, `t${i}b`)])}
            {[0, 1, 2].flatMap((r) => [eyeRow(r, `e${r}a`), eyeRow(r, `e${r}b`)])}
            {ARM_ROWS.flatMap((line, i) => [bodyLine(line, `a${i}a`), bodyLine(line, `a${i}b`)])}
            {BODY_ROWS.flatMap((line, i) => [bodyLine(line, `m${i}a`), bodyLine(line, `m${i}b`)])}
            <div style={{ animation: 'tentacle-wiggle 2s ease-in-out infinite' }}>
              {LEGS.flatMap((line, i) => [bodyLine(line, `l${i}a`), bodyLine(line, `l${i}b`)])}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
