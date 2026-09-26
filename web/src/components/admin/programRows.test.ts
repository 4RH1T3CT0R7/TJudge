import { describe, it, expect } from 'vitest';
import { buildProgramRows } from './programRows';
import type { LeaderboardEntry, Program, Team } from '../../types';

const program = (id: string, team_id: string, status: Program['status']): Program => ({
  id, team_id, status, user_id: 'u', name: `${id}.py`, game_type: 'dilemma', language: 'python',
  version: 1, created_at: '', updated_at: '',
});
const entry = (program_id: string, team_id: string, rank: number): LeaderboardEntry => ({
  rank, program_id, program_name: `${program_id}.py`, team_id, team_name: `команда ${team_id}`,
  rating: 0, wins: 0, losses: 0, draws: 0, total_games: 0,
});
const team = (id: string, name: string) => ({ id, name }) as Team;

describe('buildProgramRows', () => {
  it('провал сборки не пропадает, играющая версия берётся из лидерборда', () => {
    const rows = buildProgramRows(
      [
        program('a2', 'A', 'failed'),    // v2 не собралась, играет v1
        program('b1', 'B', 'ready'),
        program('c1', 'C', 'compiling'), // готовой версии нет
      ],
      [entry('b1', 'B', 1), entry('a1', 'A', 2)],
      [team('A', 'Альфа'), team('B', 'Бета'), team('C', 'Гамма')],
    );

    expect(rows.map((r) => [r.program.id, r.teamName, r.playing, r.stats?.rank])).toEqual([
      ['b1', 'команда B', 'this', 1],
      ['a2', 'команда A', 'previous', 2],
      ['c1', 'Гамма', 'none', undefined],
    ]);
  });
});
