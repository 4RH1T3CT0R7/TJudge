import { describe, expect, it } from 'vitest';
import {
  etaSeconds,
  gameProgress,
  honestStandings,
  nextLiveTable,
  pageCount,
  pageSlice,
  roundSummary,
  settleGames,
} from './liveStandings';
import type { CrossGameLeaderboardEntry, Game, MatchRound } from '../types';

const T0 = '2026-09-27T10:00:00Z';

const round = (game_type: string, total: number, pending: number, running = 0, created_at = T0): MatchRound => ({
  round_number: 1,
  game_type,
  total_matches: total,
  completed_count: total - pending - running,
  pending_count: pending,
  running_count: running,
  failed_count: 0,
  wins1: 0,
  wins2: 0,
  created_at,
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

const places = (rows: { entry: CrossGameLeaderboardEntry; place: number | null; total: number }[]) =>
  rows.map((r) => [r.entry.team_name, r.place, r.total]);

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

  it('без идущих игр держит порядок сервера', () => {
    expect(honestStandings(entries, new Set()).map((r) => r.place)).toEqual([1, 2, 3]);
  });

  it('частичные суммы идущей игры без прошлого итога в место не входят', () => {
    expect(places(honestStandings(entries, new Set(['b'])))).toEqual([
      ['Альфа', 1, 300],
      ['Бета', 2, 100],
      ['Гамма', null, 0],
    ]);
  });

  it('у идущей игры в место входит её прошлый итог', () => {
    const before = [entry('Бета', 1, { a: [100, 1, 18], b: [50, 1, 18] }), entry('Альфа', 2, { a: [300, 3, 18], b: [10, 0, 18] }), entry('Гамма', 3, { b: [900, 9, 18] })];
    const settled = settleGames(new Map(), before, new Set());
    expect(places(honestStandings(entries, new Set(['b']), settled))).toEqual([
      ['Гамма', 1, 900],
      ['Альфа', 2, 310],
      ['Бета', 3, 150],
    ]);
  });

  it('равным - общее место и порядок сервера', () => {
    const rows = honestStandings([entry('Я', 1, { a: [0, 0, 2] }), entry('А', 2, { a: [0, 0, 2] }), entry('Б', 3, { a: [-5, 0, 2] })], new Set());
    expect(places(rows)).toEqual([
      ['Я', 1, 0],
      ['А', 1, 0],
      ['Б', 3, -5],
    ]);
  });
});

describe('nextLiveTable', () => {
  const games = [{ id: 'a', name: 'a' }, { id: 'b', name: 'b' }] as Game[];
  const T1 = '2026-09-27T11:00:00Z';

  it('раунд, повтор упавших и итог: места меняются только по доигранным итогам', () => {
    // до раунда: обе игры доиграны, лидер Альфа
    const t0 = nextLiveTable(null, games, [entry('Альфа', 1, { a: [300, 3, 18], b: [300, 3, 18] }), entry('Бета', 2, { a: [200, 2, 18], b: [200, 2, 18] })], [round('a', 90, 0), round('b', 90, 0)], 0);
    expect(places(t0.standings)).toEqual([['Альфа', 1, 600], ['Бета', 2, 400]]);
    expect(t0.finished).toEqual([]);

    // новый раунд b: очки b сброшены, у Беты частично больше - место держит прошлый итог
    const t1 = nextLiveTable(t0, games, [entry('Бета', 1, { a: [200, 2, 18], b: [90, 1, 2] }), entry('Альфа', 2, { a: [300, 3, 18], b: [10, 0, 2] })], [round('b', 90, 80, 0, T1), round('a', 90, 0)], 1);
    expect(places(t1.standings)).toEqual([['Альфа', 1, 600], ['Бета', 2, 400]]);
    expect(t1.finished).toEqual([]);

    // b доиграна: новые очки b входят в место, итог игры - один раз
    const final = [entry('Бета', 1, { a: [200, 2, 18], b: [900, 9, 18] }), entry('Альфа', 2, { a: [300, 3, 18], b: [100, 1, 18] })];
    const t2 = nextLiveTable(t1, games, final, [round('b', 90, 0, 0, T1), round('a', 90, 0)], 2);
    expect(places(t2.standings)).toEqual([['Бета', 1, 1100], ['Альфа', 2, 400]]);
    expect(t2.finished).toEqual(['b']);

    // повтор упавших матчей того же раунда: место не трогается, второго итога нет
    const t3 = nextLiveTable(t2, games, final, [round('b', 90, 5, 0, T1), round('a', 90, 0)], 3);
    expect(places(t3.standings)).toEqual([['Бета', 1, 1100], ['Альфа', 2, 400]]);
    const t4 = nextLiveTable(t3, games, final, [round('b', 90, 0, 0, T1), round('a', 90, 0)], 4);
    expect(t4.finished).toEqual([]);
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
