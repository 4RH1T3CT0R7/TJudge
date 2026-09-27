import { useEffect, useMemo, useRef, useState } from 'react';
import { EmptyState } from '../ui/EmptyState';
import { Segmented } from '../ui/Segmented';
import type { HeadToHeadCell } from '../../types';

interface HeadToHeadMatrixProps {
  cells: HeadToHeadCell[];
  myTeamId?: string;
  /** id команд по месту в таблице игры: в этом порядке строки и колонки. */
  order?: string[];
}

type Metric = 'wins' | 'avg';
const METRICS = [
  { value: 'wins', label: 'Победы' },
  { value: 'avg', label: 'Средние очки' },
] as const;
const VIEWS = [
  { value: 'all', label: 'Все команды' },
  { value: 'mine', label: 'Моя команда' },
] as const;
// с этого числа команд - пиксельная карта без текста: таблица 21×21 уже не помещается
const HEATMAP_FROM = 21;

interface Team {
  id: string;
  name: string;
  /** Место в таблице игры; нет - команды нет в таблице. */
  place?: number;
}

const games = (c: HeadToHeadCell) => c.wins + c.losses + c.draws;
const avg = (score: number, c: HeadToHeadCell) => Math.round(score / games(c));

// Личные встречи: строка — команда, колонка — соперник. Данные уже слиты по обеим
// ориентациям матчей (AB и BA). Два вида: доля побед (красный → зелёный) и средние
// очки строки за матч (ярче — больше, это матрица выплат для «экологии»).
export function HeadToHeadMatrix({ cells, myTeamId, order }: HeadToHeadMatrixProps) {
  const [metric, setMetric] = useState<Metric>('wins');
  const [view, setView] = useState<'all' | 'mine'>('all');

  const { teams, byPair, range } = useMemo(() => {
    const totals = new Map<string, { name: string; wins: number }>();
    const pair = new Map<string, HeadToHeadCell>();
    let lo = Infinity;
    let hi = -Infinity;
    for (const c of cells) {
      const t = totals.get(c.team_id) ?? { name: c.team_name, wins: 0 };
      t.wins += c.wins;
      totals.set(c.team_id, t);
      pair.set(`${c.team_id}:${c.opponent_id}`, c);
      if (games(c) > 0) {
        lo = Math.min(lo, c.score_for / games(c));
        hi = Math.max(hi, c.score_for / games(c));
      }
    }
    const place = new Map((order ?? []).map((id, i) => [id, i + 1]));
    const sorted: Team[] = [...totals.entries()]
      .sort(
        (a, b) =>
          (place.get(a[0]) ?? Infinity) - (place.get(b[0]) ?? Infinity) ||
          b[1].wins - a[1].wins ||
          a[1].name.localeCompare(b[1].name)
      )
      .map(([id, t]) => ({ id, name: t.name, place: place.get(id) }));
    return { teams: sorted, byPair: pair, range: { lo, hi } };
  }, [cells, order]);

  if (teams.length < 2) {
    return (
      <EmptyState command="личные встречи" hint="появятся после первых завершённых матчей минимум двух команд" />
    );
  }

  const cellOf = (row: Team, col: Team) => {
    const c = byPair.get(`${row.id}:${col.id}`);
    return c && games(c) > 0 ? c : null;
  };
  const colorOf = (c: HeadToHeadCell) => {
    if (metric === 'avg') {
      const t = range.hi > range.lo ? (c.score_for / games(c) - range.lo) / (range.hi - range.lo) : 1;
      return `rgba(139, 92, 246, ${0.08 + t * 0.72})`;
    }
    // 0 → красный, 0.5 → серый, 1 → зелёный; ничья - половина победы, одни ничьи - серые
    const rate = (c.wins + c.draws / 2) / games(c);
    if (rate === 0.5) return 'rgba(107, 114, 128, 0.25)';
    return rate > 0.5 ? `rgba(34, 197, 94, ${0.1 + (rate - 0.5) * 0.8})` : `rgba(239, 68, 68, ${0.1 + (0.5 - rate) * 0.8})`;
  };
  const describe = (row: Team, col: Team, c: HeadToHeadCell) =>
    `${row.name} против ${col.name}: ${c.wins}–${c.losses}${c.draws ? `, ничьих ${c.draws}` : ''}, очки ${c.score_for}:${c.score_against}, в среднем за матч ${avg(c.score_for, c)}:${avg(c.score_against, c)}`;
  const mine = teams.find((t) => t.id === myTeamId);

  return (
    <div>
      <div className="mb-4 flex flex-wrap gap-2">
        <Segmented label="Что показать в клетках" options={METRICS} value={metric} onChange={setMetric} />
        {mine && <Segmented label="Чьи встречи" options={VIEWS} value={view} onChange={setView} />}
      </div>

      {view === 'mine' && mine ? (
        <MyTeamTable me={mine} teams={teams} cellOf={cellOf} />
      ) : teams.length >= HEATMAP_FROM ? (
        <PixelMatrix teams={teams} cellOf={cellOf} colorOf={colorOf} describe={describe} myTeamId={myTeamId} />
      ) : (
        <SmallMatrix teams={teams} cellOf={cellOf} colorOf={colorOf} describe={describe} metric={metric} myTeamId={myTeamId} />
      )}

      {view === 'all' && (
        <p className="mt-2 font-mono text-xs text-gray-500">
          <span aria-hidden="true">{'// '}</span>
          {metric === 'wins'
            ? 'победы–поражения команды из строки над командой из колонки, третье число — ничьи; цвет — доля побед, ничья — половина победы'
            : `средние очки команды из строки за матч против колонки: ярче — больше, от ${Math.round(range.lo)} до ${Math.round(range.hi)}`}
          {order && ' · команды по месту в таблице игры'}
        </p>
      )}
    </div>
  );
}

interface GridProps {
  teams: Team[];
  cellOf: (row: Team, col: Team) => HeadToHeadCell | null;
  colorOf: (c: HeadToHeadCell) => string;
  describe: (row: Team, col: Team, c: HeadToHeadCell) => string;
  myTeamId?: string;
}

function SmallMatrix({ teams, cellOf, colorOf, describe, metric, myTeamId }: GridProps & { metric: Metric }) {
  return (
    // на телефоне матрица шире экрана: область прокрутки доступна с клавиатуры
    <div tabIndex={0} role="region" aria-label="Матрица личных встреч" className="overflow-x-auto">
      <table className="border-separate" style={{ borderSpacing: 2 }}>
        <thead>
          <tr>
            <th className="text-left text-xs font-medium text-gray-400 px-2 py-1 sticky left-0" style={{ backgroundColor: '#0a0a0b' }}>
              Команда \ Соперник
            </th>
            {teams.map((t) => (
              <th key={t.id} className="px-1 py-2 align-bottom" title={t.name}>
                <span
                  className={`block text-xs font-medium max-w-[72px] truncate ${t.id === myTeamId ? 'text-primary-300' : 'text-gray-400'}`}
                  style={{ writingMode: 'vertical-rl', transform: 'rotate(180deg)', maxHeight: 96 }}
                >
                  {t.name}
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {teams.map((row) => (
            <tr key={row.id} className={row.id === myTeamId ? 'row-mine' : undefined} aria-current={row.id === myTeamId ? 'true' : undefined}>
              <th
                className="text-left text-xs font-medium text-gray-300 px-2 py-1 whitespace-nowrap max-w-[160px] truncate sticky left-0"
                style={{ backgroundColor: '#0a0a0b' }}
                title={row.name}
              >
                {/* в узкой колонке имя обрезается, поэтому метка своей строки - в начале */}
                {row.id === myTeamId && (
                  <span className="font-mono text-primary-400">
                    <span aria-hidden="true">&gt; </span>
                    <span className="sr-only">вы: </span>
                  </span>
                )}
                {row.name}
              </th>
              {teams.map((col) => {
                if (row.id === col.id) {
                  return (
                    <td key={col.id} className="w-12 h-10 text-center text-gray-700 text-xs rounded" style={{ backgroundColor: '#111827' }}>
                      —
                    </td>
                  );
                }
                const cell = cellOf(row, col);
                if (!cell) {
                  return (
                    <td key={col.id} className="w-12 h-10 text-center text-gray-600 text-xs rounded" style={{ backgroundColor: '#111827' }}>
                      ·
                    </td>
                  );
                }
                return (
                  <td
                    key={col.id}
                    className="w-12 h-10 whitespace-nowrap px-1 text-center font-mono text-xs font-semibold tabular-nums rounded cursor-default"
                    style={{ backgroundColor: colorOf(cell), color: '#e5e7eb' }}
                    title={describe(row, col, cell)}
                  >
                    {metric === 'wins' ? `${cell.wins}–${cell.losses}${cell.draws ? `–${cell.draws}` : ''}` : avg(cell.score_for, cell)}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Пиксельная тепловая карта для больших турниров: клетка на пару без текста,
// пара под указателем или выбранная стрелками расписана строкой ниже.
function PixelMatrix({ teams, cellOf, colorOf, describe, myTeamId }: GridProps) {
  const ref = useRef<HTMLCanvasElement>(null);
  const [cur, setCur] = useState<{ r: number; c: number } | null>(null);
  const n = teams.length;
  const px = n > 60 ? 6 : 8;
  const mine = teams.findIndex((t) => t.id === myTeamId);

  useEffect(() => {
    const canvas = ref.current;
    const ctx = canvas?.getContext('2d');
    if (!canvas || !ctx) return;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = n * px * dpr;
    canvas.height = n * px * dpr;
    ctx.scale(dpr, dpr);
    teams.forEach((row, r) =>
      teams.forEach((col, c) => {
        const cell = r === c ? null : cellOf(row, col);
        ctx.fillStyle = r === c ? '#1f2937' : cell ? colorOf(cell) : '#111827';
        ctx.fillRect(c * px, r * px, px - 1, px - 1);
      })
    );
    ctx.lineWidth = 1;
    if (mine >= 0) {
      ctx.strokeStyle = '#a78bfa';
      ctx.strokeRect(0.5, mine * px - 0.5, n * px - 1, px);
      ctx.strokeRect(mine * px - 0.5, 0.5, px, n * px - 1);
    }
    if (cur) {
      ctx.strokeStyle = '#f3f4f6';
      ctx.strokeRect(cur.c * px - 0.5, cur.r * px - 0.5, px, px);
    }
  });

  const move = (dr: number, dc: number) =>
    setCur((p) => {
      const from = p ?? { r: 0, c: 0 };
      return { r: Math.max(0, Math.min(n - 1, from.r + dr)), c: Math.max(0, Math.min(n - 1, from.c + dc)) };
    });
  const KEYS: Record<string, [number, number]> = { ArrowUp: [-1, 0], ArrowDown: [1, 0], ArrowLeft: [0, -1], ArrowRight: [0, 1] };
  const row = cur && teams[cur.r];
  const col = cur && teams[cur.c];
  const cell = row && col && row !== col ? cellOf(row, col) : null;

  return (
    <div>
      <canvas
        ref={ref}
        tabIndex={0}
        role="img"
        aria-label={`Личные встречи ${n} команд: строки и колонки по месту, стрелки выбирают пару`}
        aria-describedby="h2h-pair"
        className="block aspect-square w-full cursor-crosshair [image-rendering:pixelated]"
        style={{ maxWidth: n * px }}
        onFocus={() => setCur((p) => p ?? { r: 0, c: 1 })}
        onKeyDown={(e) => {
          const d = KEYS[e.key];
          if (!d) return;
          e.preventDefault();
          move(d[0], d[1]);
        }}
        onMouseMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const at = (v: number, size: number) => Math.max(0, Math.min(n - 1, Math.floor((v / size) * n)));
          setCur({ r: at(e.clientY - rect.top, rect.height), c: at(e.clientX - rect.left, rect.width) });
        }}
      />
      <p id="h2h-pair" aria-live="polite" className="mt-2 min-h-[2.5em] font-mono text-sm text-gray-200">
        {row && col && (
          <>
            <span aria-hidden="true" className="text-primary-400">&gt; </span>
            {row === col
              ? `${row.place ? `${row.place}. ` : ''}${row.name}: против себя не играет`
              : cell
                ? describe(row, col, cell)
                : `${row.name} против ${col.name}: доигранных матчей нет`}
          </>
        )}
      </p>
    </div>
  );
}

// «Моя команда против всех»: строка на соперника, по его месту.
function MyTeamTable({ me, teams, cellOf }: { me: Team; teams: Team[]; cellOf: GridProps['cellOf'] }) {
  return (
    <div tabIndex={0} role="region" aria-label="Встречи вашей команды" className="relative overflow-x-auto">
      <table className="w-full whitespace-nowrap text-sm">
        <thead>
          <tr className="border-b border-gray-700 text-left text-gray-400">
            <th className="px-2 pb-2 text-right font-mono">#</th>
            <th className="pb-2 pr-4">Соперник</th>
            <th className="pb-2 pr-4 text-center">Победы–поражения</th>
            <th className="pb-2 pr-4 text-right">Очки</th>
            <th className="pb-2 text-right">В среднем за матч</th>
          </tr>
        </thead>
        <tbody>
          {teams
            .filter((t) => t.id !== me.id)
            .map((t) => {
              const c = cellOf(me, t);
              const tone = !c ? 'text-gray-500' : c.wins > c.losses ? 'text-green-400' : c.wins < c.losses ? 'text-red-400' : 'text-gray-300';
              return (
                <tr key={t.id} className="border-b border-gray-800 font-mono tabular-nums">
                  <td className="px-2 py-2 text-right text-gray-500">{t.place ?? '—'}</td>
                  <td className="py-2 pr-4 font-sans text-gray-200">{t.name}</td>
                  <td className={`py-2 pr-4 text-center ${tone}`}>
                    {c ? `${c.wins}–${c.losses}${c.draws ? ` (ничьих ${c.draws})` : ''}` : 'нет матчей'}
                  </td>
                  <td className="py-2 pr-4 text-right text-gray-300">{c ? `${c.score_for}:${c.score_against}` : '—'}</td>
                  <td className="py-2 text-right text-gray-300">{c ? `${avg(c.score_for, c)}:${avg(c.score_against, c)}` : '—'}</td>
                </tr>
              );
            })}
        </tbody>
      </table>
    </div>
  );
}
