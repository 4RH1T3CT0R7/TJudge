import { useMemo } from 'react';
import { useQueries, useQuery, type UseQueryResult } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { Spinner } from '../ui/Spinner';
import { tournamentInsights } from '../../utils/insights';
import type { StandingRow } from '../../utils/liveStandings';
import type { Game, HeadToHeadCell, MatchRound } from '../../types';

// итоги завершённого турнира не меняются
const STALE_MS = 5 * 60_000;
// вне компонента: TanStack пересобирает результат, только когда меняются сами запросы
const combineH2h = (results: UseQueryResult<HeadToHeadCell[]>[]) => ({
  data: results.map((r) => r.data ?? []),
  pending: results.some((r) => r.isPending),
});

// Итоги турнира: автоинсайты по личным встречам всех игр, стратегиям дилеммы и раундам.
// ponytail: считается на клиенте из head-to-head всех игр (около 2,5 МБ на игру при
// 100 командах, поэтому только у завершённого турнира); если станет тяжело -
// серверная ручка /tournaments/{id}/stats с теми же агрегатами.
export function TournamentInsights({
  tournamentId,
  games,
  rows,
  rounds,
}: {
  tournamentId: string;
  games: Game[];
  rows: StandingRow[];
  rounds: MatchRound[];
}) {
  const h2h = useQueries({
    queries: games.map((g) => ({
      queryKey: queryKeys.headToHead(tournamentId, g.id),
      queryFn: () => api.getHeadToHead(tournamentId, g.id),
      staleTime: STALE_MS,
    })),
    combine: combineH2h,
  });
  const dilemma = games.find((g) => g.name === 'dilemma');
  const strategies = useQuery({
    queryKey: queryKeys.strategies(tournamentId, dilemma?.id ?? ''),
    queryFn: () => api.getStrategyProfiles(tournamentId, dilemma!.id),
    enabled: !!dilemma,
    staleTime: STALE_MS,
  });

  const loading = h2h.pending || (!!dilemma && strategies.isPending);
  const insights = useMemo(() => {
    const places = new Map(
      rows.flatMap((r) => (r.entry.team_id && r.place !== null ? [[r.entry.team_id, { name: r.entry.team_name, place: r.place }] as const] : []))
    );
    const headToHead = new Map(games.map((g, i) => [g.id, h2h.data[i]]));
    return tournamentInsights({ places, games, headToHead, rounds, strategies: strategies.data });
  }, [rows, games, rounds, strategies.data, h2h.data]);

  if (loading) {
    return <p className="mt-8 text-sm text-gray-400"><Spinner>загрузка итогов</Spinner></p>;
  }
  if (insights.length === 0) return null;

  return (
    <section aria-labelledby="insights-title" className="mt-8">
      <h3 id="insights-title" className="mb-3 text-base font-semibold text-gray-100">Итоги турнира</h3>
      <ul className="grid gap-3 sm:grid-cols-2">
        {insights.map((i) => (
          <li key={i.label} className="rounded border border-gray-800 bg-gray-900/60 p-4">
            <p className="font-mono text-xs text-primary-400">
              <span aria-hidden="true">&gt; </span>
              {i.label}
            </p>
            <p className="mt-1 text-sm text-gray-200">{i.text}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
