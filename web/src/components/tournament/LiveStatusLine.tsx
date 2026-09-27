import { useEffect, useState, type ReactNode } from 'react';
import { etaSeconds, roundSummary, type GameProgress } from '../../utils/liveStandings';
import { getGameConfig } from '../../utils/gameConfig';
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
  /** Когда пришли раунды, по которым посчитан progress, мс. */
  progressAt: number;
  games: Game[];
  status: TournamentStatus;
  isConnected: boolean;
  isOnline: boolean;
  /** Когда данные таблицы и раундов обновились последний раз, мс. */
  updatedAt: number;
  className?: string;
}

// Строка состояния турнира в одну строку: связь, прогресс раунда с оценкой
// остатка и свежесть данных. Высота не меняется, что бы ни шло: раскладка под
// ней не прыгает. Несколько идущих игр сводятся в один сегмент «раунд»,
// по играм прогресс виден в шапке таблицы. Тикает раз в секунду, поэтому не live-регион.
export function LiveStatusLine({ progress, progressAt, games, status, isConnected, isOnline, updatedAt, className = '' }: LiveStatusLineProps) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const active = status === 'active';
  const age = updatedAt ? now - updatedAt : 0;
  // опрос и живая связь есть только у идущего турнира: у остальных данные не меняются
  const conn = !isOnline
    ? { text: '✕ нет сети', cls: 'text-red-400' }
    : !active
      ? null
      : isConnected
        ? { text: '● LIVE', cls: 'text-green-400' }
        : age > STALE_MS
          ? { text: '✕ нет связи', cls: 'text-red-400' }
          : { text: '○ опрос', cls: 'text-yellow-400' };
  const round = active ? roundSummary(progress) : null;
  const name = (type: string) => getGameConfig(type).short ?? games.find((g) => g.name === type)?.display_name ?? type;

  const segments: ReactNode[] = [];
  if (conn) segments.push(<span className={conn.cls}>{conn.text}</span>);
  if (status === 'completed') {
    segments.push(
      <span className="text-gray-300">
        <span aria-hidden="true">■ </span>турнир завершён
      </span>
    );
  } else if (status === 'pending') {
    segments.push(
      <span className="text-gray-300">
        <span aria-hidden="true">○ </span>регистрация, турнир не начат
      </span>
    );
  } else if (!round) {
    segments.push(<span className="text-gray-400">раунд не идёт</span>);
  } else {
    const single = round.live.length === 1 && round.total === round.live[0].total;
    const names = round.live.map((p) => name(p.gameType)).join(', ');
    const filled = Math.round((BAR * round.done) / Math.max(round.total, 1));
    const left = etaSeconds(round, progressAt, now);
    segments.push(
      <span className="text-gray-100" title={single ? undefined : `идут: ${names}`}>
        <span aria-hidden="true" className="text-blue-400">◐ </span>
        {single ? names : 'раунд'}{' '}
        <span aria-hidden="true" className="text-primary-400">
          {'▓'.repeat(filled)}
          <span className="text-gray-600">{'░'.repeat(BAR - filled)}</span>
        </span>{' '}
        <span className="sr-only">сыграно </span>
        {round.done}
        <span aria-hidden="true">/</span>
        <span className="sr-only"> из </span>
        {round.total}
        {!single && <span className="sr-only">, идут: {names}</span>}
        {left !== null && <span className="text-gray-400"> {eta(left)}</span>}
      </span>
    );
  }
  // свежесть важна, пока данные меняются или живой связи нет
  if (active && updatedAt > 0 && (round || !isConnected)) {
    segments.push(<span className="text-gray-400">обновлено {ago(age)} назад</span>);
  }

  return (
    <p className={`truncate font-mono tabular-nums ${className}`}>
      {segments.map((s, i) => (
        // «обновлено» - последний сегмент: на узком экране его срезает многоточие
        <span key={i}>
          {i > 0 && <span aria-hidden="true" className="text-gray-600"> │ </span>}
          {s}
        </span>
      ))}
    </p>
  );
}
