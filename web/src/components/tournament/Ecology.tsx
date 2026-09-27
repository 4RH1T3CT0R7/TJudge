import { useMemo, useState } from 'react';
import { payoffMatrix, replicate } from '../../utils/ecology';
import type { HeadToHeadCell } from '../../types';

const GENERATIONS = 200;
// категориальная шкала для тёмного фона: порядок проверен на различимость при дальтонизме,
// девятая и дальше команды - в «остальных». Одна лишняя команда выводится по имени цветом OTHER
const COLORS = ['#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#008300', '#9085e9', '#e66767'];
const OTHER = '#4b5563';
const W = 600;
const H = 200;

const pct = (x: number) => `${(x * 100).toLocaleString('ru-RU', { maximumFractionDigits: x < 0.1 ? 1 : 0 })}%`;

// «Экология» Аксельрода на данных личных встреч: как менялись бы доли стратегий
// в популяции, если бы каждое поколение размножались те, кто набирает больше очков.
export function Ecology({ cells, order, myTeamId }: { cells: HeadToHeadCell[]; order?: string[]; myTeamId?: string }) {
  const [hover, setHover] = useState<number | null>(null);
  const data = useMemo(() => {
    const names = new Map(cells.map((c) => [c.team_id, c.team_name]));
    const rest = [...names.keys()].filter((id) => !order?.includes(id)).sort();
    const ids = [...(order ?? []).filter((id) => names.has(id)), ...rest];
    const history = replicate(payoffMatrix(cells, ids), GENERATIONS);
    const final = history.at(-1) ?? [];
    const shown = ids.length === COLORS.length + 1 ? ids.length : COLORS.length;
    const top = ids.map((_, i) => i).sort((a, b) => final[b] - final[a]).slice(0, shown);
    return { ids, names, history, top };
  }, [cells, order]);

  const { ids, names, history, top } = data;
  if (ids.length < 2) return null;

  const others = ids.length - top.length;
  // слои снизу вверх: команды по итоговой доле, сверху «остальные»
  const layers = history.map((p) => {
    const vs = top.map((i) => p[i]);
    return others > 0 ? [...vs, Math.max(0, 1 - vs.reduce((s, v) => s + v, 0))] : vs;
  });
  const x = (g: number) => (g / GENERATIONS) * W;
  const bands = layers[0].map((_, k) => {
    const lower = layers.map((l) => l.slice(0, k).reduce((s, v) => s + v, 0));
    const upper = lower.map((v, g) => v + layers[g][k]);
    const pts = (vs: number[]) => vs.map((v, g) => `${x(g).toFixed(1)},${(H - v * H).toFixed(1)}`);
    return `M${pts(upper).join(' L')} L${pts(lower).reverse().join(' L')} Z`;
  });

  const gen = hover ?? GENERATIONS;
  const shares = history[gen];
  // первое место по сумме очков среди команд с личными встречами
  const leader = order?.find((id) => names.has(id));
  const survivor = ids[top[0]];
  const mineIdx = myTeamId ? ids.indexOf(myTeamId) : -1;

  return (
    <div>
      <p className="mb-3 text-sm text-gray-300">
        {survivor === leader
          ? `«${names.get(leader)}» первая и по сумме очков, и в экологии: через ${GENERATIONS} поколений у неё ${pct(history[GENERATIONS][top[0]])} популяции.`
          : `Через ${GENERATIONS} поколений больше всех популяции у «${names.get(survivor)}» — ${pct(history[GENERATIONS][top[0]])}${
              leader ? `, хотя по сумме очков первая «${names.get(leader)}»` : ''
            }.`}
      </p>
      <div className="flex gap-2">
        <div aria-hidden="true" className="flex flex-col justify-between text-right font-mono text-xs text-gray-500">
          <span>100%</span>
          <span>0</span>
        </div>
        <svg
          viewBox={`0 0 ${W} ${H}`}
          preserveAspectRatio="none"
          className="h-52 min-w-0 flex-1"
          role="img"
          aria-label={`Доли стратегий по поколениям; итог: ${top
            .slice(0, 3)
            .map((i) => `${names.get(ids[i])} ${pct(history[GENERATIONS][i])}`)
            .join(', ')}`}
          onMouseLeave={() => setHover(null)}
          onMouseMove={(e) => {
            const r = e.currentTarget.getBoundingClientRect();
            setHover(Math.max(0, Math.min(GENERATIONS, Math.round(((e.clientX - r.left) / r.width) * GENERATIONS))));
          }}
        >
          {bands.map((d, k) => (
            // тонкий контур цвета карточки - зазор между слоями
            <path key={k} d={d} fill={COLORS[k] ?? OTHER} stroke="#111827" strokeWidth="1" vectorEffect="non-scaling-stroke" />
          ))}
          {hover !== null && <line x1={x(hover)} x2={x(hover)} y1={0} y2={H} stroke="#f3f4f6" strokeWidth="1" vectorEffect="non-scaling-stroke" />}
        </svg>
      </div>
      <p aria-hidden="true" className="ml-10 mt-1 flex justify-between font-mono text-xs text-gray-500">
        <span>поколение 0</span>
        <span>{GENERATIONS}</span>
      </p>

      <ul aria-label={`Доли после поколения ${gen}`} className="mt-3 grid gap-x-6 gap-y-1 font-mono text-sm sm:grid-cols-2">
        {top.map((i, k) => (
          <li key={ids[i]} className="flex min-w-0 items-center gap-2">
            <span aria-hidden="true" className="h-3 w-3 shrink-0" style={{ background: COLORS[k] ?? OTHER }} />
            <span className={`min-w-0 flex-1 truncate font-sans ${ids[i] === myTeamId ? 'text-primary-300' : 'text-gray-200'}`}>
              {names.get(ids[i])}
            </span>
            <span className="tabular-nums text-gray-100">{pct(shares[i])}</span>
          </li>
        ))}
        {others > 0 && (
          <li className="flex items-center gap-2">
            <span aria-hidden="true" className="h-3 w-3 shrink-0" style={{ background: OTHER }} />
            <span className="flex-1 font-sans text-gray-400">остальные {others}</span>
            <span className="tabular-nums text-gray-300">
              {pct(Math.max(0, 1 - top.reduce((s, i) => s + shares[i], 0)))}
            </span>
          </li>
        )}
      </ul>
      {mineIdx >= 0 && !top.includes(mineIdx) && (
        <p className="mt-2 font-mono text-sm text-gray-300">
          <span aria-hidden="true" className="text-primary-400">&gt; </span>
          {names.get(myTeamId!)}: {pct(shares[mineIdx])}
        </p>
      )}
      <p className="mt-3 font-mono text-xs text-gray-500">
        <span aria-hidden="true">{'// '}</span>
        {hover !== null ? `поколение ${hover} · ` : ''}в поколении 0 у всех поровну; дальше доля команды растёт
        пропорционально её средним очкам против текущей смеси соперников (очки — средние за матч из личных встреч;
        против себя стратегия не играет, поэтому встречает только остальных)
      </p>
    </div>
  );
}
