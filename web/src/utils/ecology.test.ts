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
  it('средние очки за матч; против себя и без данных - null', () => {
    const cells = [cell('a', 'b', 20), cell('a', 'c', 40), cell('b', 'a', 6), cell('c', 'a', 8, 0)];
    expect(payoffMatrix(cells, ['a', 'b', 'c'])).toEqual([
      [null, 10, 20],
      [3, null, null],
      [null, null, null],
    ]);
  });

  it('доли в сумме 1, сильная стратегия вытесняет слабую', () => {
    // дилемма: всегда предающий против всегда сотрудничающего
    const history = replicate([[null, 10], [0, null]], 200);
    expect(history).toHaveLength(201);
    expect(history[0]).toEqual([0.5, 0.5]);
    for (const p of history) expect(p[0] + p[1]).toBeCloseTo(1, 9);
    expect(history[200][0]).toBeGreaterThan(0.99);
  });

  it('вывод Аксельрода: без самоигры предатель вымирает среди взаимных стратегий', () => {
    // 1000 итераций с выплатами 5/10/0/1: AllD, Tit-for-Tat, Grim, AllC
    const last = replicate(
      [
        [null, 1009, 1009, 10000],
        [999, null, 5000, 5000],
        [999, 5000, null, 5000],
        [0, 5000, 5000, null],
      ],
      200
    ).at(-1)!;
    expect(last[0]).toBeLessThan(0.01);
    expect(last[1]).toBeGreaterThan(0.3);
    expect(last[3]).toBeGreaterThan(0.1);
  });

  it('отрицательные очки (аукцион) не ломают доли', () => {
    const last = replicate([[null, 50], [-60, null]], 50).at(-1)!;
    expect(last.every((x) => x >= 0 && x <= 1)).toBe(true);
    expect(last[0]).toBeGreaterThan(last[1]);
  });
});
