import type { LeaderboardEntry, Program, Team } from '../../types';

/** Строка вкладки «Программы»: последняя версия команды и её результаты в раундах. */
export interface ProgramRow {
  program: Program;
  teamName: string;
  /** Результаты команды; нет - в раундах команда не участвует (нет готовой версии или дисквалифицирована). */
  stats?: LeaderboardEntry;
  /** Какая версия играет: эта, предыдущая готовая (последняя не собралась или собирается) или никакая. */
  playing: 'this' | 'previous' | 'none';
}

/**
 * Строки строятся от программ (последняя версия каждой команды, любой статус),
 * а не от лидерборда: иначе пропадают команды, у которых сборка не прошла.
 * Лидерборд даёт очки и версию, которая реально играет: последнюю готовую.
 */
export function buildProgramRows(programs: Program[], leaderboard: LeaderboardEntry[], teams: Team[]): ProgramRow[] {
  const statsByTeam = new Map(leaderboard.map((e) => [e.team_id, e]));
  const teamNames = new Map(teams.map((t) => [t.id, t.name]));

  return programs
    .map((program): ProgramRow => {
      const stats = program.team_id ? statsByTeam.get(program.team_id) : undefined;
      const playing = !stats ? 'none' : stats.program_id === program.id ? 'this' : 'previous';
      const teamName = stats?.team_name || (program.team_id && teamNames.get(program.team_id)) || '-';
      return { program, teamName, stats, playing };
    })
    .sort((a, b) =>
      (a.stats?.rank ?? Infinity) - (b.stats?.rank ?? Infinity) || a.teamName.localeCompare(b.teamName, 'ru'));
}
