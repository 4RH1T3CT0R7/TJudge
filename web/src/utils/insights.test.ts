import { describe, expect, it } from 'vitest';
import { tournamentInsights } from './insights';
import type { HeadToHeadCell, MatchRound } from '../types';

const cell = (a: string, b: string, wins: number, losses: number, score_for = 0): HeadToHeadCell => ({
  team_id: a,
  team_name: a,
  opponent_id: b,
  opponent_name: b,
  wins,
  losses,
  draws: 2 - wins - losses,
  score_for,
  score_against: 0,
});

const round = (game_type: string, wins1: number, wins2: number, completed_count: number): MatchRound => ({
  round_number: 1,
  game_type,
  total_matches: completed_count,
  completed_count,
  pending_count: 0,
  running_count: 0,
  failed_count: 0,
  wins1,
  wins2,
  created_at: '2026-09-27T10:00:00Z',
});

describe('tournamentInsights', () => {
  const places = new Map([
    ['A', { name: 'A', place: 1 }],
    ['B', { name: 'B', place: 2 }],
    ['C', { name: 'C', place: 3 }],
  ]);
  const games = [
    { id: 'g1', name: 'dilemma', display_name: 'Дилемма' },
    { id: 'g2', name: 'tug_of_war', display_name: 'Канат' },
  ];

  it('лидер, сенсация, урок дилеммы, кооперация, очерёдность и ничьи', () => {
    const headToHead = new Map([
      // дилемма: B выигрывает все пары, но по сумме очков последняя
      ['g1', [
        cell('A', 'B', 0, 2, 1000), cell('B', 'A', 2, 0, 1010),
        cell('A', 'C', 1, 1, 5000), cell('C', 'A', 1, 1, 5000),
        cell('B', 'C', 2, 0, 1010), cell('C', 'B', 0, 2, 1000),
      ]],
      // канат: места в игре A, B, C; C обыгрывает B, стоящую выше
      ['g2', [
        cell('A', 'B', 2, 0, 150), cell('B', 'A', 0, 2, 140),
        cell('A', 'C', 2, 0, 190), cell('C', 'A', 0, 2, 10),
        cell('B', 'C', 0, 2, 50), cell('C', 'B', 2, 0, 60),
      ]],
    ]);
    const insights = tournamentInsights({
      places,
      games,
      headToHead,
      rounds: [round('dilemma', 5, 5, 20), round('tug_of_war', 14, 6, 20)],
      strategies: [
        { team_id: 'A', team_name: 'A', matches: 4, cooperation: 0.9, niceness: 1, retaliation: 1, forgiveness: 1, provocability: 1 },
        { team_id: 'B', team_name: 'B', matches: 4, cooperation: 0.1, niceness: 0, retaliation: 1, forgiveness: 0, provocability: 1 },
      ],
    });
    expect(insights.map((i) => i.text)).toEqual([
      '«A» уступила: «B» (Дилемма 2–0)',
      '«C» (3-е место в игре «Канат») обыграла «B» (2-е), 2–0',
      '«B» выиграла 4 из 4 матчей в игре «Дилемма» — больше всех, но по сумме очков только 3-е место: здесь важна сумма очков, а не победы в парах',
      '«A» сотрудничала в 90% ходов дилеммы, меньше всех — «B», 10%',
      'в игре «Канат» игрок 1 выигрывает 70% решённых матчей',
      'больше всего ничьих в игре «Дилемма»: 50% сыгранных матчей',
    ]);
  });

  it('победа над лидером и победа в дилемме - не сенсация', () => {
    const headToHead = new Map([
      ['g1', [cell('C', 'B', 2, 0, 10), cell('B', 'C', 0, 2, 50)]],
      ['g2', [cell('C', 'A', 2, 0, 10), cell('A', 'C', 0, 2, 50)]],
    ]);
    const labels = tournamentInsights({ places, games, headToHead, rounds: [] }).map((i) => i.label);
    expect(labels).not.toContain('главная сенсация');
  });

  it('без данных инсайтов нет, непобеждённый лидер', () => {
    expect(tournamentInsights({ places, games, headToHead: new Map(), rounds: [] })).toEqual([]);
    const insights = tournamentInsights({ places, games, headToHead: new Map([['g1', [cell('A', 'B', 2, 0)]]]), rounds: [] });
    expect(insights[0].text).toBe('«A» не проиграла ни одной личной встречи');
  });
});
