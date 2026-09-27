import { describe, expect, it } from 'vitest';
import { payoffMatrix, replicate } from './ecology';
import type { HeadToHeadCell } from '../types';

const cell = (team_id: string, opponent_id: string, score_for: number, games = 2): HeadToHeadCell => ({
  team_id,
  team_name: team_id,
  opponent_id,
  opponent_name: opponent_id,
  wins: 0,
  losses: 0,
  draws: games,
  score_for,
  score_against: 0,
});

describe('ecology', () => {
  it('средние очки за матч, против себя и без данных - средние по строке', () => {
    const cells = [cell('a', 'b', 20), cell('a', 'c', 40), cell('b', 'a', 6), cell('c', 'a', 8, 0)];
    expect(payoffMatrix(cells, ['a', 'b', 'c'])).toEqual([
      [15, 10, 20],
      [3, 3, 3],
      [0, 0, 0],
    ]);
  });

  it('доли в сумме 1, сильная стратегия вытесняет слабую', () => {
    // дилемма: всегда предающий против всегда сотрудничающего
    const history = replicate([[1, 10], [0, 5]], 200);
    expect(history).toHaveLength(201);
    expect(history[0]).toEqual([0.5, 0.5]);
    for (const p of history) expect(p[0] + p[1]).toBeCloseTo(1, 9);
    expect(history[200][0]).toBeGreaterThan(0.99);
  });

  it('отрицательные очки (аукцион) не ломают доли', () => {
    const last = replicate([[-10, 50], [-60, 0]], 50).at(-1)!;
    expect(last.every((x) => x >= 0 && x <= 1)).toBe(true);
    expect(last[0]).toBeGreaterThan(last[1]);
  });
});
