import { useEffect, useState } from 'react';
import type { UseQueryResult } from '@tanstack/react-query';
import { nextLiveTable, type LiveTable } from '../utils/liveStandings';
import type { CrossGameLeaderboardEntry, Game, MatchRound } from '../types';

/**
 * Честная таблица из пары «таблица + раунды». Запросы идут отдельно, а место
 * зависит от обоих: какие игры идут, решают раунды. Пара принимается, только
 * когда ни один из двух не перечитывается: обновления запускают их вместе,
 * и рендер со свежими раундами и старой таблицей (или наоборот) дал бы ложную
 * перестановку. До этого остаётся прежняя пара; null - первой ещё нет.
 *
 * Опрос пары (pollInterval) - здесь, одним таймером на оба запроса: таймер
 * refetchInterval у каждого запроса перезапускается после его ответа, и два
 * опроса расходятся на разницу во времени ответа.
 */
export function useLiveTable(
  games: Game[] | undefined,
  leaderboard: UseQueryResult<CrossGameLeaderboardEntry[]>,
  rounds: UseQueryResult<MatchRound[]>,
  pollInterval: number | false = false
): LiveTable | null {
  const refetchLeaderboard = leaderboard.refetch;
  const refetchRounds = rounds.refetch;
  useEffect(() => {
    if (!pollInterval) return;
    const t = setInterval(() => {
      if (document.hidden) return;
      void refetchLeaderboard();
      void refetchRounds();
    }, pollInterval);
    return () => clearInterval(t);
  }, [pollInterval, refetchLeaderboard, refetchRounds]);

  const [table, setTable] = useState<LiveTable | null>(null);
  const idle = !leaderboard.isFetching && !rounds.isFetching;
  if (
    idle &&
    games &&
    leaderboard.data &&
    rounds.data &&
    (table?.entries !== leaderboard.data || table.rounds !== rounds.data || table.games !== games)
  ) {
    setTable(nextLiveTable(table, games, leaderboard.data, rounds.data, rounds.dataUpdatedAt));
  }
  return table;
}
