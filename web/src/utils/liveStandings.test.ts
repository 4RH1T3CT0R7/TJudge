import { describe, expect, it } from 'vitest';
import { etaSeconds, gameProgress, honestStandings, pageCount, pageSlice, roundSummary } from './liveStandings';
import type { CrossGameLeaderboardEntry, MatchRound } from '../types';

const round = (game_type: string, total: number, pending: number, running = 0): MatchRound => ({
  round_number: 1,
  game_type,
  total_matches: total,
  completed_count: total - pending - running,
  pending_count: pending,
  running_count: running,
  failed_count: 0,
  wins1: 0,
  wins2: 0,
  created_at: '2026-09-27T10:00:00Z',
});

const entry = (name: string, rank: number, games: Record<string, [number, number, number]>): CrossGameLeaderboardEntry => {
  const game_ratings = Object.fromEntries(
    Object.entries(games).map(([id, [rating, wins, total_games]]) => [
      id,
      { game_id: id, game_name: id, rating, wins, losses: 0, draws: 0, total_games },
    ])
  );
  const sum = Object.values(games).reduce((s, [r]) => s + r, 0);
  return { rank, team_id: name, team_name: name, program_id: name, program_name: name, game_ratings, total_rating: sum, total_wins: 0, total_losses: 0, total_games: 0 };
};

describe('gameProgress', () => {
  it('берёт последний раунд игры и считает матчи команды', () => {
    const p = gameProgress([round('tug', 90, 49), round('tug', 90, 0)]).get('tug')!;
    expect(p).toMatchObject({ done: 41, total: 90, perTeam: 18, live: true });
  });
});

describe('etaSeconds', () => {
  it('считает темп по всему раунду, а не по игре, которая ждала в очереди', () => {
    // «Запустить все»: раунды трёх игр созданы разом, очередь играет их по очереди
    const progress = gameProgress([round('a', 90, 0), round('b', 90, 45), round('c', 90, 90)]);
    const r = roundSummary(progress)!;
    expect(r).toMatchObject({ done: 135, total: 270 });
    const at = r.startedAt + 60_000;
    // 135 матчей за 60 с: оставшиеся 135 - ещё 60 с, из них 10 с уже прошло после данных
    expect(etaSeconds(r, at, at)).toBe(60);
    expect(etaSeconds(r, at, at + 10_000)).toBe(50);
    // меньше 10 % сыграно - оценивать рано
    expect(etaSeconds({ ...r, done: 20 }, at, at)).toBeNull();
  });

  it('раунд без идущих игр не сводится', () => {
    expect(roundSummary(gameProgress([round('a', 90, 0)]))).toBeNull();
  });
});

describe('honestStandings', () => {
  // игра a доиграна, b идёт: у Беты больше частичных очков, но место по a
  const entries = [
    entry('Бета', 1, { a: [100, 1, 18], b: [900, 5, 10] }),
    entry('Альфа', 2, { a: [300, 3, 18], b: [100, 1, 4] }),
    entry('Гамма', 3, { b: [50, 0, 2] }),
  ];

  it('без идущих игр отдаёт места сервера', () => {
    expect(honestStandings(entries, new Set()).map((r) => r.place)).toEqual([1, 2, 3]);
  });

  it('частичные суммы идущей игры не меняют место', () => {
    const rows = honestStandings(entries, new Set(['b']));
    expect(rows.map((r) => [r.entry.team_name, r.place, r.total])).toEqual([
      ['Альфа', 1, 300],
      ['Бета', 2, 100],
      ['Гамма', null, 0],
    ]);
  });

  it('равенство решает название, а не частичные суммы', () => {
    const rows = honestStandings([entry('Я', 1, { b: [9, 1, 1] }), entry('А', 2, { b: [1, 0, 1] })], new Set(['b']));
    expect(rows.map((r) => [r.entry.team_name, r.place])).toEqual([
      ['А', null],
      ['Я', null],
    ]);
  });
});

describe('страницы табло', () => {
  it('на второй странице закреплена тройка лидеров', () => {
    const rows = Array.from({ length: 25 }, (_, i) => i + 1);
    expect(pageCount(25, 10)).toBe(4);
    expect(pageSlice(rows, 10, 0)).toEqual({ pinned: [], body: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] });
    expect(pageSlice(rows, 10, 1)).toEqual({ pinned: [1, 2, 3], body: [11, 12, 13, 14, 15, 16, 17] });
    expect(pageSlice(rows, 10, 3).body).toEqual([25]);
  });
});
