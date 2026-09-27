// Честный live: место - по сумме последних доигранных итогов каждой игры.
// Частичная сумма идущей игры зависит от того, чьи матчи очередь сыграла раньше,
// и дала бы ложные смены лидера. Поэтому пока игра идёт, в место входит её
// прошлый итог (если страница его застала), а новые очки видны в таблице с
// пометкой «предварительно». Объяснение для участников - в /help#place.
import type { CrossGameLeaderboardEntry, Game, GameRatingInfo, MatchRound } from '../types';

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
  /** Место по доигранным итогам; null - у команды нет ни одного доигранного матча. */
  place: number | null;
  /** Сумма, победы, поражения и матчи доигранных итогов: то, по чему место. */
  total: number;
  wins: number;
  losses: number;
  games: number;
}

export const teamKey = (e: CrossGameLeaderboardEntry) => e.team_id ?? e.program_id;

/** Последний доигранный итог игры: id игры → ключ команды → её очки. */
export type Settled = Map<string, Map<string, GameRatingInfo>>;

export function liveGameIds(games: Game[], progress: Map<string, GameProgress>): Set<string> {
  return new Set(games.filter((g) => progress.get(g.name)?.live).map((g) => g.id));
}

/** Итоги не идущих игр обновляются, у идущей остаётся прошлый, если он был. */
export function settleGames(prev: Settled, entries: CrossGameLeaderboardEntry[], live: Set<string>): Settled {
  const next = new Map(prev);
  for (const id of new Set(entries.flatMap((e) => Object.keys(e.game_ratings)))) {
    if (live.has(id)) continue;
    next.set(id, new Map(entries.filter((e) => e.game_ratings[id]).map((e) => [teamKey(e), e.game_ratings[id]])));
  }
  return next;
}

export function honestStandings(
  entries: CrossGameLeaderboardEntry[],
  live: Set<string>,
  settled: Settled = new Map()
): StandingRow[] {
  const scored = entries.map((entry, index) => {
    const row = { entry, index, total: 0, wins: 0, losses: 0, games: 0 };
    for (const [gameId, current] of Object.entries(entry.game_ratings)) {
      const r = live.has(gameId) ? settled.get(gameId)?.get(teamKey(entry)) : current;
      if (!r) continue;
      row.total += r.rating;
      row.wins += r.wins;
      row.losses += r.losses;
      row.games += r.total_games;
    }
    return row;
  });
  // при равенстве суммы и побед место общее, а порядок - как у сервера:
  // иначе строки менялись бы местами от частичных сумм
  scored.sort(
    (a, b) => Number(b.games > 0) - Number(a.games > 0) || b.total - a.total || b.wins - a.wins || a.index - b.index
  );
  let place = 0;
  return scored.map((s, i) => {
    const prev = scored[i - 1];
    if (!prev || prev.total !== s.total || prev.wins !== s.wins) place = i + 1;
    return { entry: s.entry, place: s.games > 0 ? place : null, total: s.total, wins: s.wins, losses: s.losses, games: s.games };
  });
}

/** Согласованная пара «таблица + раунды» и всё, что из неё следует. */
export interface LiveTable {
  entries: CrossGameLeaderboardEntry[];
  rounds: MatchRound[];
  games: Game[];
  progress: Map<string, GameProgress>;
  /** id идущих игр. */
  live: Set<string>;
  standings: StandingRow[];
  /** Игры (game_type), чей раунд доигран в этом обновлении: баннер «$ итог:». */
  finished: string[];
  settled: Settled;
  /** Раунды, уже виденные доигранными: повтор упавших матчей второго итога не даёт. */
  doneRounds: Set<string>;
  /** Когда пришли раунды, мс: от этого момента считается темп. */
  at: number;
}

const roundKey = (p: GameProgress) => `${p.gameType}@${p.startedAt}`;

export function nextLiveTable(
  prev: LiveTable | null,
  games: Game[],
  entries: CrossGameLeaderboardEntry[],
  rounds: MatchRound[],
  at: number
): LiveTable {
  const progress = gameProgress(rounds);
  const live = liveGameIds(games, progress);
  const settled = settleGames(prev?.settled ?? new Map(), entries, live);
  const done = [...progress.values()].filter((p) => !p.live);
  return {
    entries,
    rounds,
    games,
    progress,
    live,
    standings: honestStandings(entries, live, settled),
    finished: prev ? done.filter((p) => !prev.doneRounds.has(roundKey(p))).map((p) => p.gameType) : [],
    settled,
    doneRounds: new Set([...(prev?.doneRounds ?? []), ...done.map(roundKey)]),
    at,
  };
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
