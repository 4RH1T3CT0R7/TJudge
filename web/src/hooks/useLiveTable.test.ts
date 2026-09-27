// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest';
import { renderHook } from '@testing-library/react';
import type { UseQueryResult } from '@tanstack/react-query';
import { useLiveTable } from './useLiveTable';
import type { CrossGameLeaderboardEntry, Game, MatchRound } from '../types';

const games = [{ id: 'g', name: 'tug' }] as Game[];
const round = (pending: number): MatchRound =>
  ({ game_type: 'tug', total_matches: 2, pending_count: pending, running_count: 0, created_at: '2026-09-27T10:00:00Z' }) as MatchRound;
const lb = (a: number, b: number): CrossGameLeaderboardEntry[] =>
  [
    ['A', a],
    ['B', b],
  ].map(([name, rating]) => ({
    team_id: name,
    team_name: name,
    program_id: name,
    game_ratings: { g: { rating, wins: 0, losses: 0, draws: 0, total_games: 2 } },
  })) as unknown as CrossGameLeaderboardEntry[];

const q = <T,>(data: T, isFetching = false) => ({ data, isFetching, dataUpdatedAt: 1 }) as unknown as UseQueryResult<T>;

describe('useLiveTable', () => {
  it('не смешивает свежие раунды со старой таблицей', () => {
    const { result, rerender } = renderHook(({ l, r }) => useLiveTable(games, l, r), {
      initialProps: { l: q(lb(5, 1)), r: q([round(1)]) },
    });
    const during = result.current!;
    expect(during.live.has('g')).toBe(true);

    // раунды уже говорят «доиграно», таблица с частичными суммами ещё перечитывается
    rerender({ l: q(lb(5, 1), true), r: q([round(0)]) });
    expect(result.current).toBe(during);

    rerender({ l: q(lb(1, 9)), r: q([round(0)]) });
    expect(result.current!.live.size).toBe(0);
    expect(result.current!.standings.map((s) => [s.entry.team_name, s.place])).toEqual([
      ['B', 1],
      ['A', 2],
    ]);
    expect(result.current!.finished).toEqual(['tug']);
  });
});
