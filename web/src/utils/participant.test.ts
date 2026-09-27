import { describe, it, expect } from 'vitest';
import { crashStats, pairTotals, playingVersion, uploadBlockReason } from './participant';
import type { Match, Program } from '../types';

const program = (version: number, status: Program['status']) =>
  ({ id: `v${version}`, version, status }) as Program;

const match = (p1: string, p2: string, fields: Partial<Match>) =>
  ({ id: `${p1}-${p2}`, program1_id: p1, program2_id: p2, created_at: '', ...fields }) as Match;

describe('playingVersion', () => {
  it('новая версия не собралась - играет старая собранная; у дисквалифицированной - никакая', () => {
    const list = [program(1, 'ready'), program(2, 'ready'), program(3, 'failed')];
    expect(playingVersion(list, false)?.version).toBe(2);
    expect(playingVersion(list, true)).toBeNull();
  });
});

describe('uploadBlockReason', () => {
  const active = { status: 'active' as const };
  const team = { is_disqualified: false };
  const manual = { auto_round_enabled: false, round_completed: false };
  const games = [{ game_name: 'dilemma', game_display_name: 'Дилемма' }];
  const round = (pending: number) => ({ game_type: 'dilemma', total_matches: 90, pending_count: pending, running_count: 0 });

  it('открыта, пока ничего не идёт; авто-раунд не закрывает даже во время матчей', () => {
    expect(uploadBlockReason(active, team, manual, games, [round(0)])).toBeNull();
    expect(uploadBlockReason(active, team, { ...manual, auto_round_enabled: true }, games, [round(53)])).toBeNull();
  });

  it('идущий раунд другой игры, завершённый раунд, дисквалификация', () => {
    expect(uploadBlockReason(active, team, manual, games, [round(53)])).toBe(
      'идёт раунд «Дилемма», сыграно 37/90: загрузка откроется, когда он закончится',
    );
    expect(uploadBlockReason(active, team, { ...manual, round_completed: true }, games, [])).toContain('раунд этой игры завершён');
    expect(uploadBlockReason(active, { is_disqualified: true }, manual, games, [])).toBe('команда дисквалифицирована');
  });
});

describe('crashStats', () => {
  it('считает только падения своей версии, системный сбой - не падение', () => {
    const matches = [
      match('me', 'a', { status: 'failed', error_code: 1, winner: 2 }), // упала своя, игрок 1
      match('b', 'me', { status: 'failed', error_code: 1, winner: 2 }), // упал соперник
      match('c', 'me', { status: 'failed', error_code: 1, winner: 0 }), // сбой воркера
      match('me', 'd', { status: 'completed', winner: 1 }),
    ];
    const s = crashStats(matches, 'me');
    expect(s?.crashed.map((m) => m.id)).toEqual(['me-a']);
    expect(s?.played).toBe(3);
    expect(s?.side).toBe(1);
    expect(crashStats(matches.slice(1), 'me')).toBeNull();
  });
});

describe('pairTotals', () => {
  it('пара AB и BA: очки и победы левой стороны в обеих ролях', () => {
    const ab = match('me', 'x', { status: 'completed', score1: 30, score2: 10, winner: 1 });
    const ba = match('x', 'me', { status: 'completed', score1: 25, score2: 5, winner: 1 });
    const t = pairTotals([ab, ba], (m) => (m.program1_id === 'me' ? 1 : 2));
    expect(t).toEqual({ leftTotal: 35, rightTotal: 35, wins: 1, losses: 1, draws: 0, finished: 2 });
  });
});
