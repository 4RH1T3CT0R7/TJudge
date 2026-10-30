// Место в турнире - по сумме мест команды в играх, меньше лучше. Честный live:
// места в игре считаются по её последнему доигранному итогу. Частичные очки
// идущей игры зависят от того, чьи матчи очередь сыграла раньше, и дали бы
// ложные смены лидера. Поэтому пока игра идёт, в место входит её прошлый итог
// (если страница его застала), а новые очки видны в таблице с пометкой
// «предварительно». Без прошлого итога (первый раунд) место в игре
// предварительное: по очкам за сыгранный матч. Объяснение - в /help#place.
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
  /** Место по итогам игр (у идущей без прошлого итога - предварительно); null - матчей нет. */
  place: number | null;
  /** Сумма мест в играх по тем же итогам, что и место, меньше лучше. */
  placeSum: number;
  /** Место в каждой учтённой игре: id игры -> место. Не сыгравшая игру команда - после последнего. */
  gamePlaces: Record<string, number>;
  /** Игры, где место поставлено «после последнего»: команда в них не играла. */
  missedGames: Set<string>;
  /** Победы, поражения и матчи итогов, по которым место. */
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
    // таблица отдаёт и нули у не игравших программ. снимок до первого раунда
    // итогом не считается: иначе с ним первый раунд шёл бы без мест
    if (!entries.some((e) => (e.game_ratings[id]?.total_games ?? 0) > 0)) {
      next.delete(id);
      continue;
    }
    next.set(id, new Map(entries.filter((e) => e.game_ratings[id]).map((e) => [teamKey(e), e.game_ratings[id]])));
  }
  return next;
}

export function honestStandings(
  entries: CrossGameLeaderboardEntry[],
  live: Set<string>,
  settled: Settled = new Map()
): StandingRow[] {
  const rows = entries.map((entry, index) => ({
    entry,
    index,
    placeSum: 0,
    gamePlaces: {} as Record<string, number>,
    missedGames: new Set<string>(),
    wins: 0,
    // победы для равных сумм: у неполной игры - доля побед, число зависело бы
    // от того, сколько матчей команды очередь уже сыграла
    tieWins: 0,
    losses: 0,
    games: 0,
  }));
  for (const gameId of new Set(entries.flatMap((e) => Object.keys(e.game_ratings)))) {
    // итог, по которому место: у идущей игры - прошлый доигранный, если он был.
    // без него - текущий неполный, и сравниваются очки и победы за матч: у
    // команд к этому моменту сыграно разное число матчей
    const prevResult = live.has(gameId) ? settled.get(gameId) : undefined;
    const partial = live.has(gameId) && !prevResult;
    const played = rows.flatMap((row) => {
      const r = prevResult ? prevResult.get(teamKey(row.entry)) : row.entry.game_ratings[gameId];
      if (!r || r.total_games === 0) return [];
      const per = partial ? r.total_games : 1;
      return [{ row, r, score: r.rating / per, winsKey: r.wins / per }];
    });
    if (played.length === 0) continue;
    played.sort((a, b) => b.score - a.score || b.winsKey - a.winsKey);
    played.forEach(({ row, r, score, winsKey }, i) => {
      const prev = played[i - 1];
      row.gamePlaces[gameId] = prev && prev.score === score && prev.winsKey === winsKey ? prev.row.gamePlaces[gameId] : i + 1;
      row.wins += r.wins;
      row.tieWins += winsKey;
      row.losses += r.losses;
      row.games += r.total_games;
    });
    // не сыгравшая игру команда - после последнего: иначе пропуск игры уменьшал бы сумму.
    // в неполной игре это ещё не сыгравшая, а не пропустившая
    for (const row of rows) {
      if (row.gamePlaces[gameId] === undefined) {
        row.gamePlaces[gameId] = played.length + 1;
        if (!partial) row.missedGames.add(gameId);
      }
      row.placeSum += row.gamePlaces[gameId];
    }
  }
  // при равенстве суммы мест и побед место общее, а порядок - как у сервера:
  // иначе строки менялись бы местами от частичных итогов
  // доли побед двух неполных игр складываются с погрешностью float
  const sameWins = (a: number, b: number) => Math.abs(a - b) < 1e-9;
  rows.sort(
    (a, b) =>
      Number(b.games > 0) - Number(a.games > 0) ||
      a.placeSum - b.placeSum ||
      (sameWins(a.tieWins, b.tieWins) ? 0 : b.tieWins - a.tieWins) ||
      a.index - b.index
  );
  let place = 0;
  return rows.map((s, i) => {
    const prev = rows[i - 1];
    if (!prev || prev.placeSum !== s.placeSum || !sameWins(prev.tieWins, s.tieWins)) place = i + 1;
    return {
      entry: s.entry,
      place: s.games > 0 ? place : null,
      placeSum: s.placeSum,
      gamePlaces: s.gamePlaces,
      missedGames: s.missedGames,
      wins: s.wins,
      losses: s.losses,
      games: s.games,
    };
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
