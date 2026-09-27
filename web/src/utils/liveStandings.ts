// Честный live: место считается только по доигранным играм. Частичные суммы
// идущей игры зависят от того, чьи матчи очередь сыграла раньше, и дают ложные
// смены лидера; поэтому они видны в таблице с пометкой «предварительно», но место
// не меняют, пока игра не доиграна. Объяснение для участников — в /help#place.
import type { CrossGameLeaderboardEntry, Game, MatchRound } from '../types';

export interface GameProgress {
  gameType: string;
  /** Матчи раунда, которые уже не в очереди и не идут: сыграны, упали или отменены. */
  done: number;
  total: number;
  /** Матчей раунда у одной команды: обе ориентации с каждым соперником. */
  perTeam: number;
  startedAt: number;
  live: boolean;
}

// Последний раунд каждой игры: раунды приходят от новых к старым.
export function gameProgress(rounds: MatchRound[]): Map<string, GameProgress> {
  const out = new Map<string, GameProgress>();
  for (const r of rounds) {
    if (out.has(r.game_type)) continue;
    const inWork = r.pending_count + r.running_count;
    // N команд играют N·(N-1) матчей, у каждой 2·(N-1)
    const teams = Math.round((1 + Math.sqrt(1 + 4 * r.total_matches)) / 2);
    out.set(r.game_type, {
      gameType: r.game_type,
      done: r.total_matches - inWork,
      total: r.total_matches,
      perTeam: r.total_matches > 0 ? 2 * (teams - 1) : 0,
      startedAt: Date.parse(r.created_at),
      live: inWork > 0,
    });
  }
  return out;
}

export interface RoundSummary {
  live: GameProgress[];
  /** Сыграно и всего по играм раунда, включая уже доигранные. */
  done: number;
  total: number;
  startedAt: number;
}

/** Идущий раунд: игры, начатые не раньше самой ранней из идущих; null - раунд не идёт. */
export function roundSummary(progress: Map<string, GameProgress>): RoundSummary | null {
  const all = [...progress.values()];
  const live = all.filter((p) => p.live);
  if (live.length === 0) return null;
  const startedAt = Math.min(...live.map((p) => p.startedAt));
  const batch = all.filter((p) => p.startedAt >= startedAt);
  return {
    live,
    done: batch.reduce((s, p) => s + p.done, 0),
    total: batch.reduce((s, p) => s + p.total, 0),
    startedAt,
  };
}

/**
 * Остаток раунда в секундах. Очередь одна и играет игры раунда друг за другом,
 * поэтому темп общий на раунд. Темп берётся на момент данных at: между
 * обновлениями число сыгранных стоит, а время идёт. null - сыграно меньше 10 %.
 */
export function etaSeconds(r: RoundSummary, at: number, now: number): number | null {
  if (r.done === 0 || r.done < r.total / 10 || !(at > r.startedAt)) return null;
  const left = ((r.total - r.done) * (at - r.startedAt)) / r.done - (now - at);
  return Math.max(0, left / 1000);
}

export interface StandingRow {
  entry: CrossGameLeaderboardEntry;
  /** Место по доигранным играм; null - ни одна игра команды ещё не доиграна. */
  place: number | null;
  /** Сумма очков доигранных игр. */
  total: number;
}

export const teamKey = (e: CrossGameLeaderboardEntry) => e.team_id ?? e.program_id;

export function liveGameIds(games: Game[], progress: Map<string, GameProgress>): Set<string> {
  return new Set(games.filter((g) => progress.get(g.name)?.live).map((g) => g.id));
}

export function honestStandings(entries: CrossGameLeaderboardEntry[], live: Set<string>): StandingRow[] {
  // пока ничего не идёт, порядок и места сервера уже честные
  if (live.size === 0) return entries.map((entry) => ({ entry, place: entry.rank, total: entry.total_rating }));

  const scored = entries.map((entry) => {
    let total = 0;
    let wins = 0;
    let played = 0;
    for (const [gameId, r] of Object.entries(entry.game_ratings)) {
      if (live.has(gameId)) continue;
      total += r.rating;
      wins += r.wins;
      played += r.total_games;
    }
    return { entry, total, wins, played };
  });
  // равенство решает название, а не частичные суммы: иначе строки прыгали бы
  scored.sort(
    (a, b) =>
      Number(b.played > 0) - Number(a.played > 0) ||
      b.total - a.total ||
      b.wins - a.wins ||
      a.entry.team_name.localeCompare(b.entry.team_name, 'ru')
  );
  let place = 0;
  return scored.map((s, i) => {
    const prev = scored[i - 1];
    if (!prev || prev.total !== s.total || prev.wins !== s.wins) place = i + 1;
    return { entry: s.entry, total: s.total, place: s.played > 0 ? place : null };
  });
}

/** Со второй страницы табло сверху закреплены лидеры. */
export const PINNED = 3;

export function pageCount(rows: number, perPage: number): number {
  if (rows <= perPage) return 1;
  return 1 + Math.ceil((rows - perPage) / (perPage - PINNED));
}

export function pageSlice<T>(rows: T[], perPage: number, page: number): { pinned: T[]; body: T[] } {
  if (page === 0 || rows.length <= perPage) return { pinned: [], body: rows.slice(0, perPage) };
  const start = perPage + (page - 1) * (perPage - PINNED);
  return { pinned: rows.slice(0, PINNED), body: rows.slice(start, start + perPage - PINNED) };
}
