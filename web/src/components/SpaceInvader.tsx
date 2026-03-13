import { useRef, useState, useEffect, useCallback, type ReactNode } from 'react';

// --- Types ---
export type InvaderPose = 'idle' | 'handsUp' | 'dance' | 'run' | 'spin' | 'spinStop'
  | 'cry' | 'sleep' | 'fly' | 'attack' | 'shield' | 'teleport' | 'transform'
  | 'celebrate' | 'peek' | 'salute' | 'dizzy' | 'typing';

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
  colorOverride?: string | null;
}

// --- Colors (violet/purple) ---
const BODY_COLOR = '#8b5cf6';
const PUPIL_COLOR = '#ffffff';
const EYE_BG_COLOR = '#2e1065';

// --- Size map ---
const SIZE_MAP: Record<string, string> = { sm: '5px', md: '8px', lg: '12px' };
const MAX_POSE_ROWS = 30; // salute pose is tallest (6+3+2+3+1 source lines × 2)


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

function getCrossedEyeLine(lineRow: number): string {
  if (lineRow === 0) return '.X....X.';
  if (lineRow === 1) return '...XX...';
  return '.X....X.';
}

// --- Cached style objects (avoid recreating on each render) ---
const BODY_STYLE = { color: BODY_COLOR } as const;
const EYE_BG_STYLE = { color: EYE_BG_COLOR } as const;
const PUPIL_STYLE = {
  color: PUPIL_COLOR,
  textShadow: '0 0 6px rgba(255,255,255,0.8)',
} as const;
const CLOSED_EYE_STYLE = { color: BODY_COLOR, opacity: 0.7 } as const;
const CROSSED_EYE_STYLE = {
  color: '#ef4444',
  textShadow: '0 0 6px rgba(239,68,68,0.8)',
} as const;

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
    } else if (content[i] === 'X') {
      let j = i;
      while (j < content.length && content[j] === 'X') j++;
      parts.push(<span key={key++} style={CROSSED_EYE_STYLE}>{content.slice(i, j)}</span>);
      i = j;
    } else {
      let j = i;
      while (j < content.length && content[j] !== '@' && content[j] !== '-' && content[j] !== 'X') j++;
      parts.push(<span key={key++} style={EYE_BG_STYLE}>{content.slice(i, j)}</span>);
      i = j;
    }
  }
  return parts;
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
  colorOverride = null,
}: SpaceInvaderProps) {
  const prefersReducedMotion = useRef(
    typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  );

  useEffect(() => {
    const mql = window.matchMedia('(prefers-reduced-motion: reduce)');
    const handler = (e: MediaQueryListEvent) => { prefersReducedMotion.current = e.matches; };
    mql.addEventListener('change', handler);
    return () => mql.removeEventListener('change', handler);
  }, []);

  /** Returns `'none'` when the user prefers reduced motion, otherwise the given CSS animation value. */
  const animate = (value: string): string =>
    prefersReducedMotion.current ? 'none' : value;

  // Derived colors — override purple with custom color when provided
  const bodyColor = colorOverride || BODY_COLOR;
  const accentColor = colorOverride || '#a78bfa';
  const bodyStyle = colorOverride ? { color: colorOverride } as const : BODY_STYLE;

  // Helper to convert hex to rgb for rgba() usage
  const hexToRgb = (hex: string) => {
    const r = parseInt(hex.slice(1, 3), 16);
    const g = parseInt(hex.slice(3, 5), 16);
    const b = parseInt(hex.slice(5, 7), 16);
    return `${r},${g},${b}`;
  };
  const accentRgb = colorOverride ? hexToRgb(colorOverride) : '139,92,246';

  const containerRef = useRef<HTMLDivElement>(null);
  const [eyePos, setEyePos] = useState<EyePos>({ col: 3, row: 1 });
  const [pose, setPose] = useState<InvaderPose>('idle');
  const [animClass, setAnimClass] = useState('');
  const bodyRef = useRef<HTMLDivElement>(null);
  const spinContainerRef = useRef<HTMLDivElement>(null);
  const particlesRef = useRef<HTMLDivElement>(null);
  const shatteringRef = useRef(false);
  const [impactEyes, setImpactEyes] = useState<'crossed' | null>(null);
  const [impactSpeech, setImpactSpeech] = useState<string | null>(null);

  // Speech bubble fade-out: keep last text visible during opacity transition, then remove from DOM
  const lastSpeechRef = useRef<string | null>(null);
  const lastSpeechIsImpactRef = useRef(false);
  const [speechOpacity, setSpeechOpacity] = useState(0);
  const speechFadeTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [, forceSpeechUpdate] = useState(0);

  useEffect(() => {
    const text = impactSpeech || speechBubble;
    if (text) {
      clearTimeout(speechFadeTimerRef.current);
      lastSpeechRef.current = text;
      lastSpeechIsImpactRef.current = !!impactSpeech;
      setSpeechOpacity(1);
    } else {
      setSpeechOpacity(0);
      speechFadeTimerRef.current = setTimeout(() => {
        lastSpeechRef.current = null;
        forceSpeechUpdate(c => c + 1);
      }, 350);
    }
    return () => clearTimeout(speechFadeTimerRef.current);
  }, [impactSpeech, speechBubble]);

  // Use refs for state that only affects eye rendering to avoid re-render cascades
  const isHoveredRef = useRef(false);
  const cursorGoneRef = useRef(false);
  const [, forceEyeUpdate] = useState(0);

  const rafRef = useRef(0);
  const lastRef = useRef('3-1');
  const clickCountRef = useRef(0);
  const clickTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const longPressTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const pointerDownPos = useRef<{ x: number; y: number; time: number } | null>(null);
  const poseTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const idleTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

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

  // --- Impact effect: chaotic character-shaped hole via canvas mask ---
  const impactAtPoint = useCallback((clientX: number, clientY: number) => {
    const bodyEl = bodyRef.current;
    const particles = particlesRef.current;
    if (!bodyEl || !particles || shatteringRef.current) return;
    shatteringRef.current = true;

    const bodyRect = bodyEl.getBoundingClientRect();
    const relX = clientX - bodyRect.left;
    const relY = clientY - bodyRect.top;

    // Monospace font metrics
    const fontSizePx = parseFloat(SIZE_MAP[size] || SIZE_MAP.md);
    const charW = fontSizePx * 0.6;

    // Hole & scatter params
    const holeRadius = size === 'sm' ? 14 : size === 'lg' ? 30 : 20;
    const scatterDist = size === 'sm' ? 30 : size === 'lg' ? 70 : 50;
    const pFontSize = fontSizePx + 'px'; // Must match invader body font size exactly

    const SPEECHES = ['// SEGFAULT', 'Error 500!', 'core dumped', 'panic()', '// \u0431\u043e\u043b\u044c\u043d\u043e!', 'FATAL ERROR', '{ hp: 0 }'];
    const NEON_COLORS = ['#a78bfa', '#4ade80', '#ffffff', '#67e8f9', '#c084fc', '#34d399'];

    // Recursively collect leaf text-row divs (handles wrapper divs for legs/dance)
    function getLeafRows(el: HTMLElement): HTMLElement[] {
      const out: HTMLElement[] = [];
      for (const child of Array.from(el.children)) {
        if (child.tagName !== 'DIV') continue;
        const div = child as HTMLElement;
        const hasDivKids = Array.from(div.children).some(c => c.tagName === 'DIV');
        if (hasDivKids) out.push(...getLeafRows(div));
        else out.push(div);
      }
      return out;
    }

    // Extract characters with chaotic jagged edge (not a perfect circle)
    const rows = getLeafRows(bodyEl);
    const innerR = holeRadius * 0.6;
    const outerR = holeRadius * 1.25;
    const extracted: { char: string; x: number; y: number; jw: number; jh: number }[] = [];

    for (const row of rows) {
      const rr = row.getBoundingClientRect();
      const text = row.textContent || '';
      const rowCY = rr.top - bodyRect.top + rr.height / 2;
      const rowTop = rr.top - bodyRect.top;
      for (let c = 0; c < text.length; c++) {
        const ch = text[c];
        if (ch === ' ' || ch === '.') continue;
        const cx = c * charW + charW / 2;
        const dx = cx - relX;
        const dy = rowCY - relY;
        const dist = Math.sqrt(dx * dx + dy * dy);

        let include = false;
        if (dist < innerR) {
          include = true; // core: always
        } else if (dist < outerR) {
          // edge: probabilistic → jagged boundary
          const prob = 1 - (dist - innerR) / (outerR - innerR);
          include = Math.random() < prob * 0.8 + 0.15;
        }
        if (include) {
          extracted.push({
            char: ch, x: cx, y: rowTop,
            // Per-char size jitter for irregular cutouts
            jw: charW * (1 + Math.random() * 0.35),
            jh: fontSizePx * (1 + Math.random() * 0.3),
          });
        }
      }
    }

    if (extracted.length === 0) {
      shatteringRef.current = false;
      return;
    }

    // --- Canvas mask: character-shaped cutouts (not a circle) ---
    const canvas = document.createElement('canvas');
    canvas.width = Math.ceil(bodyRect.width);
    canvas.height = Math.ceil(bodyRect.height);
    const ctx = canvas.getContext('2d')!;

    const buildMask = (holes: typeof extracted) => {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.globalCompositeOperation = 'source-over';
      ctx.fillStyle = 'white';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.globalCompositeOperation = 'destination-out';
      ctx.fillStyle = 'black';
      for (const h of holes) {
        ctx.fillRect(h.x - h.jw / 2, h.y - (h.jh - fontSizePx) / 2, h.jw, h.jh);
      }
      ctx.globalCompositeOperation = 'source-over';
      const url = canvas.toDataURL();
      bodyEl.style.setProperty('mask-image', `url(${url})`);
      bodyEl.style.setProperty('-webkit-mask-image', `url(${url})`);
      bodyEl.style.setProperty('mask-size', '100% 100%');
      bodyEl.style.setProperty('-webkit-mask-size', '100% 100%');
    };

    const clearMask = () => {
      bodyEl.style.removeProperty('mask-image');
      bodyEl.style.removeProperty('-webkit-mask-image');
      bodyEl.style.removeProperty('mask-size');
      bodyEl.style.removeProperty('-webkit-mask-size');
    };

    // --- Phase 1 (0ms): hole + particles + speech + shake ---
    buildMask(extracted);

    for (const ec of extracted) {
      const span = document.createElement('span');
      const color = NEON_COLORS[Math.floor(Math.random() * NEON_COLORS.length)];
      const angle = Math.atan2(ec.y + fontSizePx / 2 - relY, ec.x - relX) + (Math.random() - 0.5) * 0.5;
      const dist = scatterDist * (0.7 + Math.random() * 0.6);
      const tx = Math.cos(angle) * dist;
      const ty = Math.sin(angle) * dist;
      const rot = (Math.random() - 0.5) * 540;
      span.textContent = ec.char;
      span.style.cssText = [
        `position:absolute;left:${ec.x}px;top:${ec.y}px`,
        `color:${color};font-family:'Courier New',monospace;font-size:${pFontSize}`,
        'font-weight:bold;line-height:1',
        `text-shadow:0 0 8px ${color},0 0 16px ${color}80`,
        'pointer-events:none;z-index:25',
        `--px:${tx}px;--py:${ty}px;--pr:${rot}deg`,
        'animation:char-burst 0.35s cubic-bezier(0.22,1,0.36,1) both',
      ].join(';');
      particles.appendChild(span);
    }

    const speech = SPEECHES[Math.floor(Math.random() * SPEECHES.length)];
    setImpactSpeech(speech);
    setAnimClass('animate-shake');

    // --- Phase 2 (400ms): crossed eyes ---
    setTimeout(() => setImpactEyes('crossed'), 400);

    // --- Phase 3 (900ms): 3 DRAMATICALLY different healing modes ---
    const healMode = Math.floor(Math.random() * 3);

    // Dynamic timing based on mode
    const recoveryTime = healMode === 1 ? 1700 : 1300;
    const cleanupTime = healMode === 1 ? 2000 : 1600;

    setTimeout(() => {
      const els = Array.from(particles.children) as HTMLElement[];

      if (healMode === 0) {
        // ═══ T-1000: particles fly BACK smoothly, hole fills edges→center ═══
        els.forEach(p => { p.style.animation = 'char-return 0.4s cubic-bezier(0.22,1,0.36,1) both'; });
        const sorted = [...extracted].sort((a, b) => {
          const da = Math.hypot(a.x - relX, a.y + fontSizePx / 2 - relY);
          const db = Math.hypot(b.x - relX, b.y + fontSizePx / 2 - relY);
          return db - da; // farthest first → heals first
        });
        const steps = 6;
        const perStep = Math.ceil(sorted.length / steps);
        for (let i = 0; i < steps; i++) {
          setTimeout(() => {
            const remaining = sorted.slice(Math.min((i + 1) * perStep, sorted.length));
            if (remaining.length === 0) clearMask();
            else buildMask(remaining);
          }, i * 60);
        }

      } else if (healMode === 1) {
        // ═══ REGENERATION: particles fall with gravity + NEW chars grow in hole ═══
        // Burst particles fall under gravity
        els.forEach(p => { p.style.animation = 'char-gravity 0.6s ease-in both'; });

        // Spawn regen particles at each hole position (staggered, random order)
        const regenOrder = [...extracted].sort(() => Math.random() - 0.5);
        const stagger = Math.max(12, 400 / regenOrder.length);
        const remaining = [...extracted];

        regenOrder.forEach((ec, i) => {
          const delay = i * stagger + Math.random() * 15;
          setTimeout(() => {
            // New character pops into existence at hole position
            const regen = document.createElement('span');
            const startColor = NEON_COLORS[Math.floor(Math.random() * NEON_COLORS.length)];
            regen.textContent = ec.char;
            regen.style.cssText = [
              `position:absolute;left:${ec.x}px;top:${ec.y}px`,
              `color:${startColor};font-family:'Courier New',monospace;font-size:${pFontSize}`,
              'font-weight:bold;line-height:1',
              `text-shadow:0 0 10px ${startColor},0 0 20px ${startColor}60`,
              'pointer-events:none;z-index:20',
              'animation:char-regen 0.22s cubic-bezier(0.34,1.56,0.64,1) both',
              'transition:color 0.2s ease-out,text-shadow 0.2s ease-out',
            ].join(';');
            particles.appendChild(regen);

            // Transition neon → invader color (wound heals)
            setTimeout(() => {
              regen.style.color = bodyColor;
              regen.style.textShadow = `0 0 4px ${bodyColor}50`;
            }, 100);

            // Remove from mask + remove regen particle (original char takes over)
            setTimeout(() => {
              const idx = remaining.findIndex(r => r.x === ec.x && r.y === ec.y);
              if (idx >= 0) remaining.splice(idx, 1);
              if (remaining.length === 0) clearMask();
              else buildMask(remaining);
              regen.remove();
            }, 300);
          }, delay);
        });

      } else {
        // ═══ GLITCH: wild jitter + hole strobes on/off + sudden snap ═══
        els.forEach(p => { p.style.animation = 'char-glitch 0.4s steps(8) both'; });
        const shuffled = [...extracted].sort(() => Math.random() - 0.5);
        const flicker = [1, 0, 0.65, 0, 0.4, 0, 0.2, 0, 0.05, 0];
        flicker.forEach((factor, i) => {
          setTimeout(() => {
            if (factor <= 0) clearMask();
            else {
              const count = Math.ceil(shuffled.length * factor);
              buildMask(shuffled.slice(0, count));
            }
          }, i * 38);
        });
      }
    }, 900);

    // --- Phase 4: recovery ---
    setTimeout(() => {
      setImpactEyes(null);
      setImpactSpeech(null);
      setAnimClass('');
    }, recoveryTime);

    // --- Phase 5: cleanup ---
    setTimeout(() => {
      while (particles.firstChild) particles.removeChild(particles.firstChild);
      clearMask();
      shatteringRef.current = false;
    }, cleanupTime);
  }, [size, bodyColor]);

  // --- Click handler ---
  const handleClick = useCallback((e: React.MouseEvent) => {
    if (!interactive) return;

    // Suppress click if pointer moved (scroll/drag) or held too long (long press)
    const down = pointerDownPos.current;
    if (down) {
      const dx = e.clientX - down.x;
      const dy = e.clientY - down.y;
      if (dx * dx + dy * dy > 25) return; // >5px movement → not a tap
      if (Date.now() - down.time > 300) return; // held >300ms → not a tap
    }

    clickCountRef.current++;
    const count = clickCountRef.current;

    clearTimeout(clickTimerRef.current);
    clickTimerRef.current = setTimeout(() => { clickCountRef.current = 0; }, 2000);

    if (count >= 5) {
      clickCountRef.current = 0;
      setPose('run');
      setTimeout(() => setPose('idle'), 3000);
      return;
    }

    // Impact effect at click point
    impactAtPoint(e.clientX, e.clientY);
  }, [interactive, impactAtPoint]);

  // --- Double click ---
  const handleDoubleClick = useCallback(() => {
    if (!interactive) return;
    setPose('handsUp');
    setTimeout(() => { setPose('dance'); }, 500);
    setTimeout(() => setPose('idle'), 2500);
  }, [interactive]);

  // --- Long press / spin ---
  const isSpinningRef = useRef(false);

  const stopSpin = useCallback(() => {
    if (!isSpinningRef.current) return;
    isSpinningRef.current = false;

    const el = spinContainerRef.current;
    if (el) {
      // Read current rotation from the running animation
      const cs = getComputedStyle(el);
      const mx = cs.transform;
      let angle = 0;
      if (mx && mx !== 'none') {
        const vals = mx.match(/matrix\(([^)]+)\)/);
        if (vals) {
          const [a, b] = vals[1].split(',').map(Number);
          angle = Math.atan2(b, a) * (180 / Math.PI);
        }
      }
      // Freeze at current angle, smoothly finish to next full turn
      el.style.animation = 'none';
      el.style.transform = `rotate(${angle}deg)`;
      let target = (Math.floor(angle / 360) + 1) * 360;
      let remaining = target - angle;
      // If too little left (<60°), add another full turn to avoid a jerky micro-rotation
      if (remaining < 60) { target += 360; remaining += 360; }
      // Duration proportional to remaining angle (spin speed = 720°/s → ease-out ~half)
      const dur = Math.min(0.7, Math.max(0.2, remaining / 500));
      requestAnimationFrame(() => {
        el.style.transition = `transform ${dur}s ease-out`;
        el.style.transform = `rotate(${target}deg)`;
        setTimeout(() => {
          el.style.transition = '';
          el.style.transform = '';
          el.style.animation = '';
          setPose('idle');
        }, dur * 1000 + 20);
      });
    }
    setPose('spinStop');
  }, []);

  const handlePointerDown = useCallback((e: React.PointerEvent) => {
    if (!interactive) return;
    pointerDownPos.current = { x: e.clientX, y: e.clientY, time: Date.now() };
    longPressTimerRef.current = setTimeout(() => {
      isSpinningRef.current = true;
      setPose('spin');
    }, 500);
  }, [interactive]);

  const handlePointerUp = useCallback(() => {
    clearTimeout(longPressTimerRef.current);
    stopSpin();
  }, [stopSpin]);

  // Global pointerup — catches release even when cursor is outside the invader
  useEffect(() => {
    const onGlobalUp = () => {
      clearTimeout(longPressTimerRef.current);
      stopSpin();
    };
    document.addEventListener('pointerup', onGlobalUp);
    return () => document.removeEventListener('pointerup', onGlobalUp);
  }, [stopSpin]);

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
    if (impactEyes === 'crossed') return 'crossed';
    if (eyeOverride === 'closed') return 'closed';
    if (eyeOverride === 'sad') return 'sad';
    if (eyeOverride === 'wide') return 'wide';
    if (cursorGoneRef.current) return 'sad';
    if (isHoveredRef.current) return 'wide';
    return 'normal';
  })();

  // --- Build rows ---
  const bodyLine = (text: string, key: string) => (
    <div key={key} style={bodyStyle}>{text}</div>
  );

  const eyeRow = (lineRow: number, key: string) => {
    let leftEye: string;
    let rightEye: string;

    if (eyeState === 'closed') {
      leftEye = getClosedEyeLine(lineRow);
      rightEye = getClosedEyeLine(lineRow);
    } else if (eyeState === 'crossed') {
      leftEye = getCrossedEyeLine(lineRow);
      rightEye = getCrossedEyeLine(lineRow);
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
        <span style={bodyStyle}>{EYE_PRE[lineRow]}</span>
        {renderEyeSegment(leftEye)}
        <span style={bodyStyle}>{EYE_MID}</span>
        {renderEyeSegment(rightEye)}
        <span style={bodyStyle}>{EYE_SUF[lineRow]}</span>
      </div>
    );
  };

  const renderIdlePose = () => (
    <>
      {IDLE_TOP.flatMap((line, i) => [bodyLine(line, `t${i}a`), bodyLine(line, `t${i}b`)])}
      {[0, 1, 2].flatMap((r) => [eyeRow(r, `e${r}a`), eyeRow(r, `e${r}b`)])}
      {IDLE_ARM_ROWS.flatMap((line, i) => [bodyLine(line, `a${i}a`), bodyLine(line, `a${i}b`)])}
      {IDLE_BODY_ROWS.flatMap((line, i) => [bodyLine(line, `m${i}a`), bodyLine(line, `m${i}b`)])}
      <div style={{ animation: animate('tentacle-wiggle 2s ease-in-out infinite') }}>
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
    <div style={{ animation: animate('wave-hands 0.4s ease-in-out infinite') }}>
      {renderHandsUpPose()}
    </div>
  );

  const renderRunPose = () => (
    <>
      {IDLE_TOP.flatMap((line, i) => [bodyLine(line, `rt${i}a`), bodyLine(line, `rt${i}b`)])}
      {[0, 1, 2].flatMap((r) => [eyeRow(r, `re${r}a`), eyeRow(r, `re${r}b`)])}
      {IDLE_ARM_ROWS.flatMap((line, i) => [bodyLine(line, `ra${i}a`), bodyLine(line, `ra${i}b`)])}
      {IDLE_BODY_ROWS.flatMap((line, i) => [bodyLine(line, `rm${i}a`), bodyLine(line, `rm${i}b`)])}
      <div style={{ animation: animate('run-legs 0.2s ease-in-out infinite') }}>
        {IDLE_LEGS.flatMap((line, i) => [bodyLine(line, `rl${i}a`), bodyLine(line, `rl${i}b`)])}
      </div>
    </>
  );

  // Effect size scales with invader size
  const effectFontSize = size === 'sm' ? '10px' : size === 'lg' ? '16px' : '12px';

  const renderCryPose = () => (
    <div>
      {IDLE_TOP.flatMap((line, i) => [bodyLine(line, `ct${i}a`), bodyLine(line, `ct${i}b`)])}
      {[0, 1, 2].flatMap((r) => [eyeRow(r, `ce${r}a`), eyeRow(r, `ce${r}b`)])}
      {/* Zero-height anchor right below eye rows — tears fall from here */}
      <div style={{ position: 'relative', height: 0, overflow: 'visible', zIndex: 10 }}>
        {[
          { left: '35%', delay: '0s', ch: ';' },
          { left: '63%', delay: '0s', ch: ';' },
          { left: '35%', delay: '0.7s', ch: ',' },
          { left: '63%', delay: '0.7s', ch: ',' },
          { left: '36%', delay: '1.4s', ch: '.' },
          { left: '64%', delay: '1.4s', ch: '.' },
        ].map((t, i) => (
          <span key={`tear-${i}`} style={{
            position: 'absolute', top: 0, left: t.left,
            color: '#60a5fa', fontFamily: "'Courier New', Consolas, monospace",
            fontSize: effectFontSize, fontWeight: 'bold', lineHeight: 1,
            textShadow: '0 0 6px rgba(96,165,250,0.7)',
            animation: animate('tear-fall 1.4s ease-in infinite'),
            animationDelay: t.delay, pointerEvents: 'none',
          }}>{t.ch}</span>
        ))}
      </div>
      {IDLE_ARM_ROWS.flatMap((line, i) => [bodyLine(line, `ca${i}a`), bodyLine(line, `ca${i}b`)])}
      {IDLE_BODY_ROWS.flatMap((line, i) => [bodyLine(line, `cm${i}a`), bodyLine(line, `cm${i}b`)])}
      <div style={{ animation: animate('tentacle-wiggle 2s ease-in-out infinite') }}>
        {IDLE_LEGS.flatMap((line, i) => [bodyLine(line, `cl${i}a`), bodyLine(line, `cl${i}b`)])}
      </div>
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
            color: accentColor,
            fontFamily: 'monospace',
            fontWeight: 'bold',
            opacity: 0.8,
            animation: animate('zzz-float 2s ease-out infinite'),
            animationDelay: `${i * 0.5}s`,
          }}
        >
          {ch}
        </span>
      ))}
    </div>
  );

  const renderFlyPose = () => (
    <div style={{ animation: animate('fly-drift 2s ease-in-out infinite'), position: 'relative' }}>
      {renderHandsUpPose()}
      {/* ASCII flame exhaust */}
      {[
        { left: '35%', delay: '0s', ch: '^', color: '#ef4444' },
        { left: '47%', delay: '0.15s', ch: '*', color: '#f59e0b' },
        { left: '59%', delay: '0.3s', ch: '^', color: '#ef4444' },
      ].map((f, i) => (
        <span
          key={`flame-${i}`}
          style={{
            position: 'absolute',
            bottom: '-0.8em',
            left: f.left,
            color: f.color,
            fontFamily: "'Courier New', Consolas, monospace",
            fontSize: effectFontSize,
            fontWeight: 'bold',
            lineHeight: 1,
            textShadow: `0 0 6px ${f.color}80`,
            opacity: 0.9,
            animation: animate('particle-fly 0.5s ease-out infinite'),
            animationDelay: f.delay,
            '--px': '0px',
            '--py': '15px',
            pointerEvents: 'none',
          } as React.CSSProperties}
        >
          {f.ch}
        </span>
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
          animation: animate('fade-in 0.2s ease-out'),
          whiteSpace: 'pre',
        }}
      >
        {'>>>--->'}
      </div>
    </div>
  );

  const renderShieldPose = () => (
    <div style={{
      animation: animate('shield-pulse 1.5s ease-in-out infinite'),
      position: 'relative',
      borderRadius: '8px',
      padding: '2px',
    }}>
      {renderIdlePose()}
      {/* ASCII shield brackets */}
      <span style={{
        position: 'absolute', top: '50%', left: '-1.5em', transform: 'translateY(-50%)',
        color: '#4ade80', fontFamily: "'Courier New', Consolas, monospace",
        fontSize: effectFontSize, fontWeight: 'bold', lineHeight: 1,
        textShadow: '0 0 8px rgba(74,222,128,0.7)', pointerEvents: 'none',
      }}>[</span>
      <span style={{
        position: 'absolute', top: '50%', right: '-1.5em', transform: 'translateY(-50%)',
        color: '#4ade80', fontFamily: "'Courier New', Consolas, monospace",
        fontSize: effectFontSize, fontWeight: 'bold', lineHeight: 1,
        textShadow: '0 0 8px rgba(74,222,128,0.7)', pointerEvents: 'none',
      }}>]</span>
      {/* Shield border overlay */}
      <div style={{
        position: 'absolute', inset: '-4px', borderRadius: '8px',
        border: '1.5px solid rgba(74,222,128,0.4)',
        pointerEvents: 'none',
        animation: animate('shield-border-pulse 1.5s ease-in-out infinite'),
      }} />
    </div>
  );

  const renderTeleportPose = () => (
    <div style={{ animation: animate('teleport-glitch 0.8s ease-in-out') }}>
      {renderIdlePose()}
    </div>
  );

  const renderTransformPose = () => (
    <div style={{ filter: colorFilter || undefined }}>
      {renderIdlePose()}
    </div>
  );

  const renderCelebratePose = () => (
    <div style={{ position: 'relative' }}>
      {renderHandsUpPose()}
      {/* Confetti particles */}
      {Array.from({ length: 12 }).map((_, i) => {
        const chars = ['*', '+', '.', '~', '^'];
        const colors = ['#fbbf24', '#4ade80', '#a78bfa', '#60a5fa', '#f472b6'];
        return (
          <span
            key={`confetti-${i}`}
            style={{
              position: 'absolute',
              top: '-0.5em',
              left: `${10 + (i / 12) * 80}%`,
              color: colors[i % colors.length],
              fontFamily: "'Courier New', Consolas, monospace",
              fontSize: effectFontSize,
              fontWeight: 'bold',
              pointerEvents: 'none',
              animation: animate('char-confetti 1.8s ease-out infinite'),
              animationDelay: `${i * 0.15}s`,
            }}
          >
            {chars[i % chars.length]}
          </span>
        );
      })}
    </div>
  );

  const renderPeekPose = () => (
    <div style={{
      clipPath: 'inset(0 0 0 50%)',
      position: 'relative',
      paddingLeft: '25%',
    }}>
      {renderIdlePose()}
    </div>
  );

  const SALUTE_TOP = [
    body(4, true)+SP4+SP4+SP4+SP4+SP4+SP4+SP4+SP4+SP4+body(4, true),
    SP4+body(4, true)+SP4+SP4+SP4+SP4+SP4+SP4+SP4+body(4, true)+SP4,
    SP4+SP4+body(4, true)+'+**+'+SP4+SP4+SP4+'+**+'+SP4+SP4+SP4+SP4,
    SP4+SP4+SP4+'+**+'+SP4+SP4+SP4+'+**+'+SP4+SP4+SP4,
    SP4+SP4+body(28, true)+SP4+SP4,
    SP4+body(36, true)+SP4,
  ];

  const renderSalutePose = () => (
    <>
      {SALUTE_TOP.flatMap((line, i) => [bodyLine(line, `st${i}a`), bodyLine(line, `st${i}b`)])}
      {[0, 1, 2].flatMap((r) => [eyeRow(r, `se${r}a`), eyeRow(r, `se${r}b`)])}
      {IDLE_ARM_ROWS.flatMap((line, i) => [bodyLine(line, `sa${i}a`), bodyLine(line, `sa${i}b`)])}
      {IDLE_BODY_ROWS.flatMap((line, i) => [bodyLine(line, `sm${i}a`), bodyLine(line, `sm${i}b`)])}
      {IDLE_LEGS.flatMap((line, i) => [bodyLine(line, `sl${i}a`), bodyLine(line, `sl${i}b`)])}
    </>
  );

  const renderDizzyPose = () => (
    <div style={{ animation: animate('wobble 1s ease-in-out infinite'), position: 'relative' }}>
      {renderIdlePose()}
      {/* Spinning stars */}
      {['*', '+', '*'].map((ch, i) => (
        <span
          key={`star-${i}`}
          style={{
            position: 'absolute',
            top: `${5 + i * 8}%`,
            right: `${-5 + i * 12}%`,
            color: '#fbbf24',
            fontFamily: 'monospace',
            fontSize: '10px',
            fontWeight: 'bold',
            animation: `spin-invader ${1.5 + i * 0.3}s linear infinite`,
            opacity: 0.8,
          }}
        >
          {ch}
        </span>
      ))}
    </div>
  );

  const renderTypingPose = () => (
    <div style={{ position: 'relative' }}>
      {renderIdlePose()}
      {/* Keyboard */}
      <div style={{
        position: 'absolute',
        bottom: '-1.2em',
        left: '50%',
        transform: 'translateX(-50%)',
        color: '#6b7280',
        fontFamily: "'Courier New', Consolas, monospace",
        fontSize: effectFontSize,
        fontWeight: 'bold',
        whiteSpace: 'pre',
        animation: animate('typing-hands 0.3s steps(2) infinite'),
        pointerEvents: 'none',
      }}>
        [====]
      </div>
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
      case 'celebrate': return renderCelebratePose();
      case 'peek': return renderPeekPose();
      case 'salute': return renderSalutePose();
      case 'dizzy': return renderDizzyPose();
      case 'typing': return renderTypingPose();
      case 'spin':
      case 'spinStop':
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
      {/* Speech bubble with fade-in/fade-out via opacity transition */}
      {lastSpeechRef.current && (
        <div
          style={{
            position: 'absolute',
            top: '-2em',
            left: '50%',
            transform: 'translateX(-50%)',
            whiteSpace: 'nowrap',
            background: lastSpeechIsImpactRef.current ? 'rgba(127,29,29,0.92)' : 'rgba(17,24,39,0.92)',
            color: lastSpeechIsImpactRef.current ? '#fca5a5' : accentColor,
            fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
            fontSize: '11px',
            padding: '4px 10px',
            borderRadius: '6px',
            border: lastSpeechIsImpactRef.current ? '1px solid rgba(239,68,68,0.4)' : `1px solid rgba(${accentRgb},0.3)`,
            pointerEvents: 'none',
            zIndex: 30,
            opacity: speechOpacity,
            transition: 'opacity 0.3s ease',
          }}
        >
          {lastSpeechRef.current}
          <div
            style={{
              position: 'absolute',
              bottom: '-4px',
              left: '50%',
              transform: 'translateX(-50%) rotate(45deg)',
              width: '8px',
              height: '8px',
              background: lastSpeechIsImpactRef.current ? 'rgba(127,29,29,0.92)' : 'rgba(17,24,39,0.92)',
              borderRight: lastSpeechIsImpactRef.current ? '1px solid rgba(239,68,68,0.4)' : `1px solid rgba(${accentRgb},0.3)`,
              borderBottom: lastSpeechIsImpactRef.current ? '1px solid rgba(239,68,68,0.4)' : `1px solid rgba(${accentRgb},0.3)`,
            }}
          />
        </div>
      )}

      {/* Single float animation container — glow via filter on container, NOT text-shadow per char */}
      <div ref={spinContainerRef} style={{
        animation: pose === 'spin'
          ? 'spin-invader 0.5s linear infinite'
          : pose === 'spinStop'
            ? 'none'
            : 'invader-float 3s ease-in-out infinite',
        willChange: 'transform',
      }}>
        <div style={{
          animation: animate('invader-glow 2.5s ease-in-out infinite'),
          willChange: 'filter',
          position: 'relative',
        }}>
          {/* Particle container for flying character debris */}
          <div
            ref={particlesRef}
            style={{
              position: 'absolute',
              inset: 0,
              pointerEvents: 'none',
              zIndex: 25,
              overflow: 'visible',
            }}
          />
          <div
            ref={bodyRef}
            style={{
              fontFamily: "'Courier New', Consolas, 'Liberation Mono', monospace",
              fontSize: SIZE_MAP[size] || SIZE_MAP.md,
              lineHeight: 1,
              whiteSpace: 'pre',
              userSelect: 'none',
              cursor: interactive ? 'pointer' : 'default',
              minHeight: `${MAX_POSE_ROWS * parseFloat(SIZE_MAP[size] || SIZE_MAP.md)}px`,
            }}
          >
            {renderPose()}
          </div>
        </div>
      </div>
    </div>
  );
}
