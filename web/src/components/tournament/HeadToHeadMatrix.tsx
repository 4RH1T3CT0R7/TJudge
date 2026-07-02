import { useMemo } from 'react';
import type { HeadToHeadCell } from '../../types';

interface HeadToHeadMatrixProps {
  cells: HeadToHeadCell[];
}

// Тепловая карта личных встреч: строка — команда, колонка — соперник.
// Цвет ячейки — win-rate (красный → зелёный), в ячейке счёт побед-поражений.
// Данные приходят уже слитыми по обеим ориентациям матчей (AB и BA).
export function HeadToHeadMatrix({ cells }: HeadToHeadMatrixProps) {
  const { teams, byPair } = useMemo(() => {
    const totals = new Map<string, { name: string; wins: number }>();
    const pair = new Map<string, HeadToHeadCell>();
    for (const c of cells) {
      const t = totals.get(c.team_id) ?? { name: c.team_name, wins: 0 };
      t.wins += c.wins;
      totals.set(c.team_id, t);
      pair.set(`${c.team_id}:${c.opponent_id}`, c);
    }
    const sorted = [...totals.entries()]
      .sort((a, b) => b[1].wins - a[1].wins || a[1].name.localeCompare(b[1].name))
      .map(([id, t]) => ({ id, name: t.name }));
    return { teams: sorted, byPair: pair };
  }, [cells]);

  if (teams.length < 2) {
    return (
      <div className="empty-state">
        <h3 className="empty-state-title">Матрица пока пуста</h3>
        <p className="empty-state-description">Появится после первых завершённых матчей минимум двух команд</p>
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="border-separate" style={{ borderSpacing: 2 }}>
        <thead>
          <tr>
            <th className="text-left text-xs font-medium text-gray-400 px-2 py-1 sticky left-0" style={{ backgroundColor: '#0a0a0b' }}>
              Команда \ Соперник
            </th>
            {teams.map((t) => (
              <th key={t.id} className="px-1 py-2 align-bottom" title={t.name}>
                <span
                  className="block text-xs font-medium text-gray-400 max-w-[72px] truncate"
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
            <tr key={row.id}>
              <th
                className="text-left text-xs font-medium text-gray-300 px-2 py-1 whitespace-nowrap max-w-[160px] truncate sticky left-0"
                style={{ backgroundColor: '#0a0a0b' }}
                title={row.name}
              >
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
                const cell = byPair.get(`${row.id}:${col.id}`);
                if (!cell || cell.wins + cell.losses + cell.draws === 0) {
                  return (
                    <td key={col.id} className="w-12 h-10 text-center text-gray-600 text-xs rounded" style={{ backgroundColor: '#111827' }}>
                      ·
                    </td>
                  );
                }
                const total = cell.wins + cell.losses + cell.draws;
                const rate = cell.wins / total;
                // 0 → красный, 0.5 → серый, 1 → зелёный; прозрачность мягкая.
                const bg =
                  rate >= 0.5
                    ? `rgba(34, 197, 94, ${0.10 + (rate - 0.5) * 0.8})`
                    : `rgba(239, 68, 68, ${0.10 + (0.5 - rate) * 0.8})`;
                return (
                  <td
                    key={col.id}
                    className="w-12 h-10 text-center text-xs font-semibold rounded cursor-default transition-transform hover:scale-110"
                    style={{ backgroundColor: bg, color: '#e5e7eb' }}
                    title={`${row.name} против ${col.name}: ${cell.wins} побед, ${cell.losses} поражений${cell.draws ? `, ${cell.draws} ничьих` : ''} (счёт ${cell.score_for}:${cell.score_against})`}
                  >
                    {cell.wins}–{cell.losses}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
      <p className="text-xs text-gray-500 mt-2">
        В ячейке — победы–поражения команды из строки над командой из колонки; цвет — доля побед.
      </p>
    </div>
  );
}
