import { useMemo, useState } from 'react';

export interface ChartPoint {
  value: number;
  /** Подпись точки для тултипа (дата, номер матча и т.п.). */
  label?: string;
}

interface LineChartProps {
  points: ChartPoint[];
  height?: number;
  /** Цвет линии; по умолчанию primary-400 проекта. */
  color?: string;
  className?: string;
}

// Лёгкий SVG-график без сторонних библиотек, в неон-эстетике проекта:
// линия + градиентная заливка + hover-точки с тултипом. Ось Y подписана
// min/max, плотность точек не ограничена (path строится по всем).
export function LineChart({ points, height = 220, color = '#a78bfa', className = '' }: LineChartProps) {
  const [hover, setHover] = useState<number | null>(null);

  const W = 600; // viewBox-координаты; растягивается на ширину контейнера
  const H = height;
  const PAD = { top: 12, right: 12, bottom: 20, left: 44 };

  const { path, area, coords, min, max } = useMemo(() => {
    if (points.length === 0) {
      return { path: '', area: '', coords: [] as { x: number; y: number }[], min: 0, max: 0 };
    }
    const values = points.map((d) => d.value);
    let lo = Math.min(...values);
    let hi = Math.max(...values);
    if (lo === hi) {
      lo -= 10;
      hi += 10;
    }
    const span = hi - lo;
    const innerW = W - PAD.left - PAD.right;
    const innerH = H - PAD.top - PAD.bottom;
    const stepX = points.length > 1 ? innerW / (points.length - 1) : 0;

    const cs = points.map((d, i) => ({
      x: PAD.left + (points.length > 1 ? i * stepX : innerW / 2),
      y: PAD.top + innerH - ((d.value - lo) / span) * innerH,
    }));
    const line = cs.map((c, i) => `${i === 0 ? 'M' : 'L'}${c.x.toFixed(1)},${c.y.toFixed(1)}`).join(' ');
    const areaPath =
      line +
      ` L${cs[cs.length - 1].x.toFixed(1)},${(H - PAD.bottom).toFixed(1)}` +
      ` L${cs[0].x.toFixed(1)},${(H - PAD.bottom).toFixed(1)} Z`;
    return { path: line, area: areaPath, coords: cs, min: lo, max: hi };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [points, H]);

  if (points.length === 0) {
    return (
      <div className={`flex items-center justify-center text-sm text-gray-500 ${className}`} style={{ height }}>
        Нет данных для графика
      </div>
    );
  }

  const hovered = hover !== null ? points[hover] : null;
  const hoveredCoord = hover !== null ? coords[hover] : null;

  return (
    <div className={`relative ${className}`}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="w-full"
        style={{ height }}
        preserveAspectRatio="none"
        onMouseLeave={() => setHover(null)}
        onMouseMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const x = ((e.clientX - rect.left) / rect.width) * W;
          let nearest = 0;
          let best = Infinity;
          coords.forEach((c, i) => {
            const d = Math.abs(c.x - x);
            if (d < best) {
              best = d;
              nearest = i;
            }
          });
          setHover(nearest);
        }}
        role="img"
        aria-label="График изменения рейтинга"
      >
        <defs>
          <linearGradient id="lc-fill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity="0.25" />
            <stop offset="100%" stopColor={color} stopOpacity="0" />
          </linearGradient>
        </defs>

        {/* Сетка: min / max */}
        <line x1={PAD.left} x2={W - PAD.right} y1={PAD.top} y2={PAD.top} stroke="#374151" strokeDasharray="4 4" strokeWidth="0.5" />
        <line x1={PAD.left} x2={W - PAD.right} y1={H - PAD.bottom} y2={H - PAD.bottom} stroke="#374151" strokeWidth="0.5" />
        <text x={PAD.left - 6} y={PAD.top + 4} textAnchor="end" fontSize="11" fill="#6b7280">{max}</text>
        <text x={PAD.left - 6} y={H - PAD.bottom + 4} textAnchor="end" fontSize="11" fill="#6b7280">{min}</text>

        <path d={area} fill="url(#lc-fill)" />
        <path d={path} fill="none" stroke={color} strokeWidth="2" strokeLinejoin="round" style={{ filter: `drop-shadow(0 0 4px ${color}66)` }} />

        {hoveredCoord && (
          <>
            <line x1={hoveredCoord.x} x2={hoveredCoord.x} y1={PAD.top} y2={H - PAD.bottom} stroke="#4b5563" strokeWidth="0.5" />
            <circle cx={hoveredCoord.x} cy={hoveredCoord.y} r="4" fill={color} style={{ filter: `drop-shadow(0 0 6px ${color})` }} />
          </>
        )}
      </svg>

      {hovered && hoveredCoord && (
        <div
          className="absolute pointer-events-none px-2 py-1 rounded-md text-xs whitespace-nowrap z-10"
          style={{
            left: `${(hoveredCoord.x / W) * 100}%`,
            top: Math.max(0, (hoveredCoord.y / H) * height - 36),
            transform: 'translateX(-50%)',
            background: '#111827',
            border: '1px solid #374151',
            color: '#e5e7eb',
          }}
        >
          <span className="font-semibold" style={{ color }}>{hovered.value}</span>
          {hovered.label && <span className="text-gray-400 ml-1.5">{hovered.label}</span>}
        </div>
      )}
    </div>
  );
}

// Спарклайн: миниатюра тренда для строк таблиц (без осей и тултипов).
export function Sparkline({ points, width = 96, height = 28, color = '#a78bfa' }: {
  points: number[];
  width?: number;
  height?: number;
  color?: string;
}) {
  const path = useMemo(() => {
    if (points.length < 2) return '';
    let lo = Math.min(...points);
    let hi = Math.max(...points);
    if (lo === hi) {
      lo -= 1;
      hi += 1;
    }
    const stepX = (width - 4) / (points.length - 1);
    return points
      .map((v, i) => {
        const x = 2 + i * stepX;
        const y = 2 + (height - 4) * (1 - (v - lo) / (hi - lo));
        return `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(' ');
  }, [points, width, height]);

  if (points.length < 2) return null;

  const trendUp = points[points.length - 1] >= points[0];
  const stroke = trendUp ? '#4ade80' : color;

  return (
    <svg width={width} height={height} aria-hidden="true" className="shrink-0">
      <path d={path} fill="none" stroke={stroke} strokeWidth="1.5" strokeLinejoin="round" opacity="0.9" />
      <circle
        cx={2 + (width - 4)}
        cy={2 + (height - 4) * (1 - (points[points.length - 1] - Math.min(...points)) / (Math.max(...points) - Math.min(...points) || 1))}
        r="2"
        fill={stroke}
      />
    </svg>
  );
}
