import { failedSide, type Side } from './explainError';
import type { Match, MatchRound, Program, Team, Tournament, TournamentGameWithDetails } from '../types';

// Правила бэкенда, которые участник видит на странице игры: какая версия
// играет, почему закрыта загрузка, здоровье версии и счёт пары от первого лица.

/** Последняя загруженная версия, в каком бы статусе она ни была. */
export const latestVersion = (list: Program[]) =>
  list.reduce<Program | null>((a, b) => (a && a.version > b.version ? a : b), null);

/** Версия, которая играет: последняя собранная у недисквалифицированной команды (storage/tournament.go latestReadyParticipants). */
export function playingVersion(programs: Program[], disqualified: boolean): Program | null {
  return disqualified ? null : latestVersion(programs.filter((p) => p.status === 'ready'));
}

/**
 * Почему загрузка закрыта, по тем же правилам, что у бэкенда (handlers/program.go):
 * турнир не идёт, команда дисквалифицирована; без авто-раунда - раунд игры завершён
 * или в турнире идут матчи любой игры. null - загрузка открыта.
 */
export function uploadBlockReason(
  tournament: Pick<Tournament, 'status'> | null,
  myTeam: Pick<Team, 'is_disqualified'>,
  gameStatus: Pick<TournamentGameWithDetails, 'auto_round_enabled' | 'round_completed'> | null,
  gamesStatus: Pick<TournamentGameWithDetails, 'game_name' | 'game_display_name'>[],
  rounds: Pick<MatchRound, 'game_type' | 'total_matches' | 'pending_count' | 'running_count'>[],
): string | null {
  if (tournament?.status === 'pending') return 'турнир ещё не начался: загрузка откроется после старта';
  if (tournament?.status === 'completed') return 'турнир завершён, загрузка закрыта';
  if (myTeam.is_disqualified) return 'команда дисквалифицирована';
  if (gameStatus?.auto_round_enabled) return null;
  if (gameStatus?.round_completed) return 'раунд этой игры завершён, новые версии не принимаются';
  const running = rounds.filter((r) => r.pending_count + r.running_count > 0);
  if (running.length === 0) return null;
  const total = running.reduce((n, r) => n + r.total_matches, 0);
  const done = running.reduce((n, r) => n + r.total_matches - r.pending_count - r.running_count, 0);
  const game = gamesStatus.find((g) => g.game_name === running[0].game_type)?.game_display_name ?? running[0].game_type;
  return `идёт раунд «${game}», сыграно ${done}/${total}: загрузка откроется, когда он закончится`;
}

export interface CrashStats {
  /** Матчи, где упала сама версия. */
  crashed: Match[];
  /** Доигранные матчи версии: сыгранные и упавшие по вине одной из сторон. */
  played: number;
  /** Сторона версии в первом упавшем матче. */
  side: Side;
}

/** Здоровье версии по её матчам; null - версия не падала. */
export function crashStats(matches: Match[], programId: string): CrashStats | null {
  const own = (m: Match): Side => (m.program1_id === programId ? 1 : 2);
  const played = matches.filter((m) => m.status === 'completed' || failedSide(m) !== null);
  const crashed = played.filter((m) => failedSide(m) === own(m));
  return crashed.length > 0 ? { crashed, played: played.length, side: own(crashed[0]) } : null;
}

/** Итог пары для левой стороны: в каждом матче она может быть и игроком 1, и игроком 2. */
export function pairTotals(matches: Match[], leftSide: (m: Match) => Side) {
  let leftTotal = 0, rightTotal = 0, wins = 0, losses = 0, draws = 0, finished = 0;
  for (const m of matches) {
    const ls = leftSide(m);
    leftTotal += (ls === 1 ? m.score1 : m.score2) ?? 0;
    rightTotal += (ls === 1 ? m.score2 : m.score1) ?? 0;
    if (m.status === 'completed' || m.status === 'failed') finished++;
    if (m.winner === ls) wins++;
    else if (m.winner === (ls === 1 ? 2 : 1)) losses++;
    else if (m.status === 'completed' && m.winner === 0) draws++;
  }
  return { leftTotal, rightTotal, wins, losses, draws, finished };
}
