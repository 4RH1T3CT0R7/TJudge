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

/** Оставшееся время раунда игры в секундах по темпу с его начала; null - оценивать рано. */
export function etaSeconds(p: GameProgress, now: number): number | null {
  const elapsed = (now - p.startedAt) / 1000;
  if (!p.live || p.done === 0 || !(elapsed > 0)) return null;
  return ((p.total - p.done) * elapsed) / p.done;
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
