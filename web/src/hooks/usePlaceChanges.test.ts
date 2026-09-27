// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { usePlaceChanges } from './usePlaceChanges';
import type { StandingRow } from '../utils/liveStandings';
import type { CrossGameLeaderboardEntry } from '../types';

const row = (team: string, place: number | null): StandingRow => ({
  entry: { team_id: team, team_name: team, program_id: team } as CrossGameLeaderboardEntry,
  place,
  total: 0,
});

describe('usePlaceChanges', () => {
  afterEach(() => vi.useRealTimers());

  it('отмечает сдвиги мест и снимает отметки через 8 с', () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(({ rows }) => usePlaceChanges(rows), {
      initialProps: { rows: [row('a', 1), row('b', 2), row('c', null)] },
    });
    expect(result.current.size).toBe(0);

    // новый массив с теми же местами - не изменение
    rerender({ rows: [row('a', 1), row('b', 2), row('c', null)] });
    expect(result.current.size).toBe(0);

    rerender({ rows: [row('b', 1), row('a', 2), row('c', 3)] });
    expect(Object.fromEntries(result.current)).toEqual({ b: 1, a: -1 });

    act(() => vi.advanceTimersByTime(8000));
    expect(result.current.size).toBe(0);
  });
});
