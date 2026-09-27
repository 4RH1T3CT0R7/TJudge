import { Link } from 'react-router-dom';
import { m } from 'motion/react';
import { DUR, EASE_OUT } from '../motion/tokens';
import { ArrowsExpandIcon } from '../icons';
import { WinnersPodium } from './WinnersPodium';
import { LEADERBOARD_VIEWS } from './helpers';
import { Segmented } from '../ui/Segmented';
import { Spinner } from '../ui/Spinner';
import { EmptyState } from '../ui/EmptyState';
import { YouMark } from '../ui/YouMark';
import { CountUp } from '../ui/CountUp';
import { PlaceHint } from './PlaceHint';
import { useRevealOnMobile } from '../../hooks/useRevealOnMobile';
import { usePlaceChanges } from '../../hooks/usePlaceChanges';
import { teamKey, type GameProgress, type StandingRow } from '../../utils/liveStandings';
import { getGameConfig } from '../../utils/gameConfig';
import type { Game } from '../../types';

// перестановка строк при смене мест (FLIP); при reduced motion MotionConfig её выключает
const ROW_MOVE = { duration: DUR.slow, ease: EASE_OUT };

// Leaderboard Tab Component
export function LeaderboardTab({
  rows,
  games,
  live,
  progress,
  showCrossGame,
  onShowCrossGameChange,
  onRefresh,
  isRefreshing,
  isCompleted,
  myTeamId,
  screenHref,
}: {
  rows: StandingRow[];
  games: Game[];
  /** id идущих игр: их очки предварительные и в место не входят. */
  live: Set<string>;
  progress: Map<string, GameProgress>;
  showCrossGame: boolean;
  onShowCrossGameChange: (value: boolean) => void;
  onRefresh: () => void;
  isRefreshing: boolean;
  isCompleted: boolean;
  myTeamId?: string;
  /** Табло для проектора; нет - кнопку не показывать. */
  screenHref?: string;
}) {
  const changes = usePlaceChanges(rows);
  return (
    <div>
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4 mb-6">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-bold text-gray-100">Рейтинг</h2>
          {isRefreshing && <Spinner />}
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            onClick={onRefresh}
            disabled={isRefreshing}
            className="btn btn-secondary"
          >
            Обновить
          </button>
          <Segmented
            label="Вид таблицы"
            options={LEADERBOARD_VIEWS}
            value={showCrossGame ? 'games' : 'total'}
            onChange={(v) => onShowCrossGameChange(v === 'games')}
          />
          {screenHref && (
            // табло для проектора: на телефоне незачем
            <Link to={screenHref} className="btn btn-secondary hidden sm:inline-flex">
              <ArrowsExpandIcon />
              Табло
            </Link>
          )}
        </div>
      </div>

      {/* Show animated podium for completed tournaments */}
      {isCompleted && rows.length >= 3 && (
        <WinnersPodium entries={rows.map((r) => r.entry)} />
      )}

      {showCrossGame ? (
        <CrossGameLeaderboardTable rows={rows} games={games} live={live} progress={progress} changes={changes} myTeamId={myTeamId} />
      ) : (
        <GeneralLeaderboardTable rows={rows} changes={changes} myTeamId={myTeamId} />
      )}
      {rows.length > 0 && (
        <PlaceHint>
          {live.size > 0
            ? 'место и сумма — по последним итогам игр: очки идущей игры (◐) предварительные и войдут в место, когда она доиграет'
            : 'место — по сумме очков всех игр с множителями, а не по числу побед'}
        </PlaceHint>
      )}
    </div>
  );
}

// ▲N / ▼N рядом с местом, пока сдвиг свежий; у строки без места стрелок нет
function PlaceShift({ delta, place }: { delta?: number; place: number | null }) {
  if (!delta || place === null) return null;
  return (
    <span className={`font-mono text-[0.7em] ${delta > 0 ? 'text-green-400' : 'text-red-400'}`}>
      <span aria-hidden="true">{delta > 0 ? '▲' : '▼'}</span>
      <span className="sr-only">{delta > 0 ? 'поднялась на' : 'опустилась на'} </span>
      {Math.abs(delta)}
    </span>
  );
}

const placeBadge = (place: number | null) =>
  place === 1 ? 'rank-badge rank-gold'
    : place === 2 ? 'rank-badge rank-silver'
      : place === 3 ? 'rank-badge rank-bronze'
        : 'rank-badge rank-default';

const rowTone = (place: number | null) =>
  place === 1 ? 'leaderboard-row-gold'
    : place === 2 ? 'leaderboard-row-silver'
      : place === 3 ? 'leaderboard-row-bronze'
        : '';

// General Leaderboard Table: место, команда, сумма доигранных игр и полоса
function GeneralLeaderboardTable({
  rows,
  changes,
  myTeamId,
}: {
  rows: StandingRow[];
  changes: Map<string, number>;
  myTeamId?: string;
}) {
  const revealMine = useRevealOnMobile<HTMLDivElement>();
  if (rows.length === 0) {
    return (
      <EmptyState command="результаты" hint="пока пусто: таблица заполнится после первых сыгранных матчей" />
    );
  }

  const maxScore = Math.max(...rows.map(r => r.total), 1);

  return (
    <div className="space-y-2">
      {rows.map((row) => {
        const { entry, place, total } = row;
        const key = teamKey(entry);
        const mine = !!myTeamId && entry.team_id === myTeamId;
        return (
          <m.div
            key={key}
            layout="position"
            transition={ROW_MOVE}
            ref={mine ? revealMine : undefined}
            aria-current={mine ? 'true' : undefined}
            className={`p-4 rounded-xl transition-colors bg-gray-800/50 border border-gray-800 ${rowTone(place)} hover:shadow-md ${mine ? 'row-mine' : ''}`}
          >
            <div className="flex items-center gap-4">
              <div className="flex w-12 shrink-0 flex-col items-center">
                <span className={placeBadge(place)}>{place ?? '–'}</span>
                <PlaceShift delta={changes.get(key)} place={place} />
              </div>

              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex items-baseline">
                      <h3 className="min-w-0 font-bold truncate text-lg text-gray-100">
                        {entry.team_name || entry.program_name}
                      </h3>
                      {mine && <YouMark />}
                    </div>
                    {/* тот же срез, что у очков: доигранные итоги */}
                    <div className="flex items-center gap-3 text-sm text-gray-400">
                      <span>{row.games} игр</span>
                      <span>•</span>
                      <span className="text-emerald-400">{row.wins}W</span>
                      <span className="text-red-400">{row.losses}L</span>
                    </div>
                  </div>

                  <div className="text-right shrink-0">
                    <div className={`font-bold tabular-nums text-3xl ${
                      place === 1 ? 'text-amber-500' :
                      place === 2 ? 'text-gray-400' :
                      place === 3 ? 'text-orange-500' :
                      'text-primary-400'
                    }`}>
                      {place === null ? '–' : <CountUp value={total} />}
                    </div>
                    <div className="text-xs text-gray-400">
                      очков
                    </div>
                  </div>
                </div>

                <div className="mt-3 h-2 bg-gray-700 rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-[width] duration-(--dur-slow) ${
                      place === 1 ? 'bg-gradient-to-r from-amber-400 to-amber-500' :
                      place === 2 ? 'bg-gradient-to-r from-gray-500 to-gray-600' :
                      place === 3 ? 'bg-gradient-to-r from-orange-400 to-orange-500' :
                      'bg-gradient-to-r from-primary-400 to-primary-500'
                    }`}
                    style={{ width: `${(Math.max(total, 0) / maxScore) * 100}%` }}
                  />
                </div>
              </div>
            </div>
          </m.div>
        );
      })}
    </div>
  );
}

// Таблица по играм. broadcast - плотность табло: кегль и высота строк от высоты
// экрана (rowHeight считает табло), без подстрок; pinned - сколько первых строк
// закреплено над страницей.
export function CrossGameLeaderboardTable({
  rows,
  games,
  live,
  progress,
  changes,
  myTeamId,
  broadcast = false,
  pinned = 0,
  rowHeight,
}: {
  rows: StandingRow[];
  games: Game[];
  live: Set<string>;
  progress: Map<string, GameProgress>;
  changes: Map<string, number>;
  myTeamId?: string;
  broadcast?: boolean;
  pinned?: number;
  rowHeight?: number;
}) {
  const revealMine = useRevealOnMobile<HTMLTableRowElement>();
  if (rows.length === 0) {
    return (
      <EmptyState command="результаты" hint="пока пусто: таблица заполнится после первых сыгранных матчей" />
    );
  }

  const cell = broadcast ? 'px-[0.8vw]' : 'px-4 py-3';
  const head = broadcast
    ? 'px-[0.8vw] h-[max(40px,7vh)] text-[clamp(14px,1.9vh,26px)] font-semibold leading-tight text-gray-400'
    : 'px-4 py-3 text-sm font-semibold uppercase tracking-wide';

  return (
    <div className={broadcast ? '' : 'overflow-x-auto card p-0'}>
      <table className={`w-full ${broadcast ? 'table-fixed text-[clamp(20px,3.2vh,44px)] leading-tight text-gray-100' : 'text-gray-100'}`}>
        <thead className={broadcast ? 'border-b border-line' : 'bg-gray-800/50'}>
          <tr>
            <th className={`${head} text-left ${broadcast ? 'w-[7%]' : ''}`}>Место</th>
            <th className={`${head} text-left ${broadcast ? 'w-[30%]' : ''}`}>Команда</th>
            {games.map((game) => {
              const isLive = live.has(game.id);
              const p = progress.get(game.name);
              return (
                <th key={game.id} className={`${head} text-center`}>
                  {broadcast ? (
                    // на табло ◐ и счётчик - отдельной строкой не мельче 14 px: название целиком
                    <>
                      <span className="block truncate">{getGameConfig(game.name).short ?? game.display_name}</span>
                      {isLive && p && (
                        <span className="block truncate font-mono font-normal text-[clamp(14px,1.6vh,22px)] text-gray-300">
                          <span aria-hidden="true" className="text-blue-400">◐ </span>
                          <span className="sr-only">предварительно, сыграно </span>
                          {p.done}/{p.total}
                        </span>
                      )}
                    </>
                  ) : (
                    <>
                      <span>
                        {isLive && <span aria-hidden="true" className="text-blue-400">◐ </span>}
                        {game.display_name}
                      </span>
                      {isLive && (
                        <span className="block font-mono font-normal normal-case tracking-normal text-[0.8em] text-gray-400">
                          предварительно
                        </span>
                      )}
                    </>
                  )}
                </th>
              );
            })}
            <th className={`${head} text-right ${broadcast ? 'w-[11%]' : ''}`}>Сумма</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => {
            const { entry, place, total } = row;
            const key = teamKey(entry);
            const mine = !!myTeamId && entry.team_id === myTeamId;
            // сдвинутые строки при перестановке непрозрачны и едут поверх стоящих,
            // поднявшиеся - поверх опустившихся: тексты строк не накладываются
            const delta = changes.get(key) ?? 0;
            return (
              <m.tr
                key={key}
                layout="position"
                transition={ROW_MOVE}
                ref={mine ? revealMine : undefined}
                aria-current={mine ? 'true' : undefined}
                style={broadcast ? { height: rowHeight } : undefined}
                className={`relative border-b ${index === pinned - 1 ? 'border-b-4 border-double border-line' : 'border-gray-700/60'} ${
                  broadcast ? '' : rowTone(place)
                } ${delta ? `${delta > 0 ? 'z-20' : 'z-10'} ${broadcast ? 'bg-[#0a0a0b]' : 'bg-gray-900'}` : ''} ${mine ? 'row-mine' : ''}`}
              >
                <td className={cell}>
                  <span className="inline-flex items-baseline gap-2">
                    {broadcast ? (
                      <span className={`font-mono font-bold tabular-nums ${
                        place === 1 ? 'text-amber-400' : place === 2 ? 'text-gray-300' : place === 3 ? 'text-orange-400' : 'text-gray-400'
                      }`}>
                        {place ?? '–'}
                      </span>
                    ) : (
                      <span className={placeBadge(place)}>{place ?? '–'}</span>
                    )}
                    <PlaceShift delta={changes.get(key)} place={place} />
                  </span>
                </td>
                <td className={`${cell} ${broadcast ? 'truncate font-bold' : ''}`}>
                  <span className={broadcast ? '' : 'font-semibold'}>
                    {entry.team_name || entry.program_name}
                  </span>
                  {mine && <YouMark />}
                </td>
                {games.map((game) => {
                  const gameRating = entry.game_ratings[game.id];
                  const isLive = live.has(game.id);
                  const perTeam = progress.get(game.name)?.perTeam;
                  return (
                    <td key={game.id} className={`${cell} text-center font-mono tabular-nums`}>
                      {gameRating ? (
                        <div>
                          <span className={isLive ? 'text-gray-400' : broadcast ? '' : 'font-bold'}>
                            <CountUp value={Math.round(gameRating.rating)} />
                          </span>
                          {!broadcast && (
                            <div className="text-xs text-gray-400">
                              {isLive ? (
                                <span title="Сыграно матчей из раунда">
                                  {gameRating.total_games}/{perTeam || '?'}
                                </span>
                              ) : (
                                <>
                                  <span className="text-emerald-500" title="Побед">{gameRating.wins}</span>
                                  <span className="mx-0.5">/</span>
                                  <span className="text-red-400" title="Поражений">{gameRating.losses}</span>
                                  <span className="mx-0.5">/</span>
                                  <span title="Ничьих">{gameRating.draws || 0}</span>
                                </>
                              )}
                            </div>
                          )}
                        </div>
                      ) : (
                        <span className="text-gray-400">-</span>
                      )}
                    </td>
                  );
                })}
                <td className={`${cell} text-right`}>
                  <span className={`font-mono font-bold tabular-nums text-primary-400 ${broadcast ? '' : 'text-lg'}`}>
                    {place === null ? '–' : <CountUp value={total} />}
                  </span>
                </td>
              </m.tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
