import { useEffect, useState } from 'react';
import { etaSeconds, type GameProgress } from '../../utils/liveStandings';
import type { Game, TournamentStatus } from '../../types';

// без живого соединения данные обновляет опрос раз в 5 с; втрое дольше - связи нет
const STALE_MS = 15_000;
const BAR = 10;

function ago(ms: number): string {
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return `${s} с`;
  if (s < 3600) return `${Math.floor(s / 60)} мин`;
  return `${Math.floor(s / 3600)} ч`;
}

function eta(sec: number): string {
  if (sec < 60) return '< 1 мин';
  const min = Math.round(sec / 60);
  return min < 60 ? `≈ ${min} мин` : `≈ ${Math.floor(min / 60)} ч ${min % 60} мин`;
}

interface LiveStatusLineProps {
  progress: Map<string, GameProgress>;
  games: Game[];
  status: TournamentStatus;
  isConnected: boolean;
  isOnline: boolean;
  /** Когда данные таблицы и раундов обновились последний раз, мс. */
  updatedAt: number;
  className?: string;
}

// Строка состояния турнира: связь, прогресс идущих игр с оценкой остатка и
// свежесть данных. Тикает раз в секунду, поэтому не live-регион.
export function LiveStatusLine({ progress, games, status, isConnected, isOnline, updatedAt, className = '' }: LiveStatusLineProps) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const age = updatedAt ? now - updatedAt : 0;
  const conn = isConnected
    ? { text: '● LIVE', cls: 'text-green-400' }
    : !isOnline
      ? { text: '✕ нет сети', cls: 'text-red-400' }
      : age > STALE_MS
        ? { text: '✕ нет связи', cls: 'text-red-400' }
        : { text: '○ опрос', cls: 'text-yellow-400' };
  const live = [...progress.values()].filter((p) => p.live);
  const name = (type: string) => games.find((g) => g.name === type)?.display_name ?? type;
  // разделитель - начало сегмента: при переносе на узком экране строка начинается с «│»
  const sep = <span aria-hidden="true" className="text-gray-600">│ </span>;

  return (
    <p className={`flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono tabular-nums ${className}`}>
      <span className={`whitespace-nowrap ${conn.cls}`}>{conn.text}</span>
      {status === 'completed' ? (
        <span className="whitespace-nowrap text-gray-300">
          {sep}
          <span aria-hidden="true">■ </span>турнир завершён
        </span>
      ) : live.length === 0 ? (
        <span className="whitespace-nowrap text-gray-400">{sep}раунд не идёт</span>
      ) : (
        live.map((p) => {
          const filled = Math.round((BAR * p.done) / Math.max(p.total, 1));
          const left = etaSeconds(p, now);
          return (
            <span key={p.gameType} className="text-gray-100">
              {sep}
              {name(p.gameType)}{' '}
              <span className="whitespace-nowrap">
                <span aria-hidden="true" className="text-primary-400">
                  {'▓'.repeat(filled)}
                  <span className="text-gray-600">{'░'.repeat(BAR - filled)}</span>
                </span>{' '}
                <span className="sr-only">сыграно </span>
                {p.done}
                <span aria-hidden="true">/</span>
                <span className="sr-only"> из </span>
                {p.total}
              </span>
              {left !== null && <span className="whitespace-nowrap text-gray-400"> {eta(left)}</span>}
            </span>
          );
        })
      )}
      {updatedAt > 0 && <span className="whitespace-nowrap text-gray-400">{sep}обновлено {ago(age)} назад</span>}
    </p>
  );
}
