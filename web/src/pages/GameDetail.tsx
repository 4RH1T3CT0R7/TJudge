import { useEffect, useState, useMemo } from 'react';
import { useParams, Link, useLocation, useSearchParams } from 'react-router-dom';
import { useQuery, keepPreviousData } from '@tanstack/react-query';
import api from '../api/client';
import { queryKeys } from '../api/queryKeys';
import {
  useTournament,
  useGame,
  useTournamentGamesStatus,
  useGameLeaderboard,
  useMyTeam,
  useMatchesByRounds,
} from '../hooks/queries';
import { useTournamentLive } from '../hooks/useTournamentLive';
import { useAuthStore } from '../store/authStore';
import { Modal } from '../components/ui/Modal';
import { Tabs } from '../components/ui/Tabs';
import { StatusLabel } from '../components/ui/StatusLabel';
import { Spinner } from '../components/ui/Spinner';
import { Segmented } from '../components/ui/Segmented';
import { MatchError } from '../components/tournament/MatchError';
import { ProgramPanel, ProgramSummary } from '../components/tournament/ProgramPanel';
import type { Side } from '../utils/explainError';
import { pairTotals } from '../utils/participant';
import { EmptyState } from '../components/ui/EmptyState';
import { ErrorState } from '../components/ui/ErrorState';
import { PageHeader } from '../components/ui/PageHeader';
import { LineChart } from '../components/ui/LineChart';
import { Markdown } from '../components/ui/Markdown';
import { YouMark } from '../components/ui/YouMark';
import { revealAndFocus, useRevealOnMobile } from '../hooks/useRevealOnMobile';
import { HeadToHeadMatrix } from '../components/tournament/HeadToHeadMatrix';
import { AutoRoundCountdown } from '../components/tournament/AutoRoundCountdown';
import { ChartBarIcon } from '../components/icons';
import { getGameConfig } from '../utils/gameConfig';
import { useTabParam } from '../hooks/useTabParam';
import type { Match } from '../types';

const TAB_IDS = ['rules', 'leaderboard', 'matches'] as const;
const MATCH_VIEWS = [
  { value: 'mine', label: 'Мои' },
  { value: 'all', label: 'Все' },
] as const;

export function GameDetail() {
  const { tournamentId, gameId } = useParams<{ tournamentId: string; gameId: string }>();
  const { isAuthenticated } = useAuthStore();
  const [activeTab, setActiveTab] = useTabParam(TAB_IDS, 'rules');

  const matchesPerPage = 20;

  // Живые обновления, как на странице турнира: WS-события инвалидируют ключи
  // игры, без WS (аноним, обрыв) у идущего турнира работает поллинг.
  const tournamentQuery = useTournament(tournamentId ?? '');
  const live = useTournamentLive({
    tournamentId: tournamentId ?? '',
    enabled: isAuthenticated,
  });

  // Базовые данные страницы
  const gameQuery = useGame(gameId ?? '');
  const gamesStatusQuery = useTournamentGamesStatus(tournamentId ?? '', { pollInterval: live.pollInterval });
  const leaderboardQuery = useGameLeaderboard(tournamentId ?? '', gameId ?? '', { pollInterval: live.pollInterval });
  const myTeamQuery = useMyTeam(tournamentId ?? '', { enabled: isAuthenticated });

  const tournament = tournamentQuery.data ?? null;
  const game = gameQuery.data ?? null;
  const myTeam = myTeamQuery.data ?? null;
  const leaderboard = leaderboardQuery.data ?? [];
  const gameStatus = useMemo(
    () => (gamesStatusQuery.data ?? []).find((gs) => gs.game_id === gameId) ?? null,
    [gamesStatusQuery.data, gameId]
  );

  // График ELO (rating_history): история подгружается лениво при открытии модалки.
  const [chartProgram, setChartProgram] = useState<{ id: string; name: string } | null>(null);
  const ratingHistoryQuery = useQuery({
    queryKey: queryKeys.ratingHistory(tournamentId ?? '', chartProgram?.id ?? ''),
    queryFn: () => api.getProgramRatingHistory(tournamentId!, chartProgram!.id),
    enabled: Boolean(tournamentId && chartProgram),
    staleTime: 30_000,
  });

  // Head-to-head матрица: грузится при открытой вкладке рейтинга.
  const headToHeadQuery = useQuery({
    queryKey: queryKeys.headToHead(tournamentId ?? '', gameId ?? ''),
    queryFn: () => api.getHeadToHead(tournamentId!, gameId!),
    enabled: Boolean(tournamentId && gameId) && activeTab === 'leaderboard',
    staleTime: 30_000,
  });

  // «Мои матчи»: участнику по умолчанию только матчи своей команды (?view=all - все),
  // ?status=failed - только упавшие (ссылка из бейджа здоровья программы).
  // Страница сбрасывается на первую при смене фильтра.
  const myTeamId = myTeam?.id;
  const [searchParams, setSearchParams] = useSearchParams();
  const showAllMatches = !myTeamId || searchParams.get('view') === 'all';
  const onlyFailed = searchParams.get('status') === 'failed';
  const matchesTeam = showAllMatches ? undefined : myTeamId;
  const filterKey = `${matchesTeam ?? ''}:${onlyFailed}`;
  const [pageState, setPageState] = useState({ key: filterKey, page: 1 });
  const currentPage = pageState.key === filterKey ? pageState.page : 1;
  const setCurrentPage = (page: number) => setPageState({ key: filterKey, page });
  const setMatchesParam = (name: 'view' | 'status', value: string | null) =>
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (value === null) next.delete(name);
        else next.set(name, value);
        return next;
      },
      { replace: true },
    );

  // Матчи с пагинацией: ключ включает страницу и фильтр, предыдущая страница
  // остаётся на экране, пока грузится новая. Total ручка не отдаёт, поэтому
  // запрашивается на один матч больше: лишний означает, что есть следующая страница.
  const matchesQuery = useQuery({
    queryKey: [
      ...queryKeys.gameMatches(tournamentId ?? '', gameId ?? ''),
      currentPage,
      matchesTeam ?? '',
      onlyFailed ? 'failed' : '',
    ] as const,
    queryFn: ({ signal }) =>
      api.getGameMatches(
        tournamentId ?? '',
        gameId ?? '',
        onlyFailed ? 'failed' : undefined,
        matchesPerPage + 1,
        (currentPage - 1) * matchesPerPage,
        signal,
        matchesTeam
      ),
    // до ответа my-team неизвестно, какой фильтр нужен
    enabled: !!tournamentId && !!gameId && !(isAuthenticated && myTeamQuery.isPending),
    placeholderData: keepPreviousData,
    refetchInterval: live.pollInterval,
  });
  // переход из бейджа здоровья программы: список матчей в поле зрения и в фокусе,
  // иначе на телефоне всё остаётся у карточки программы внизу страницы
  const location = useLocation();
  const focusMatches = (location.state as { focus?: string } | null)?.focus === 'matches';
  useEffect(() => {
    if (focusMatches) revealAndFocus(document.getElementById('game-matches'));
  }, [focusMatches, location.key]);

  const pageData = matchesQuery.data ?? [];
  const hasNextPage = pageData.length > matchesPerPage;
  const matches = hasNextPage ? pageData.slice(0, matchesPerPage) : pageData;

  // Версии команды по игре, кто бы из участников их ни загрузил: в матчах
  // играет последняя версия команды, а не последняя своя. Поллинг каждые 10с,
  // пока какая-то версия компилируется (бейдж статуса обновится сам)
  const revealMine = useRevealOnMobile<HTMLTableRowElement>();
  const roundsQuery = useMatchesByRounds(tournamentId ?? '', { pollInterval: live.pollInterval, enabled: isAuthenticated });
  const programsQuery = useQuery({
    queryKey: queryKeys.programVersions(myTeamId ?? '', gameId ?? ''),
    queryFn: () => api.getProgramVersions(myTeamId!, gameId!),
    enabled: isAuthenticated && !!myTeamId && !!gameId,
    // без WS (обрыв) итог сборки и самопроверки иначе не пришёл бы
    refetchInterval: (query) =>
      query.state.data?.some((p) => p.status === 'compiling' || p.check_status === 'pending') ? 5000 : false,
  });
  const programs = useMemo(() => programsQuery.data ?? [], [programsQuery.data]);
  const myProgramIds = useMemo(() => new Set(programs.map((p) => p.id)), [programs]);
  const isLoading =
    tournamentQuery.isPending ||
    gameQuery.isPending ||
    gamesStatusQuery.isPending ||
    leaderboardQuery.isPending ||
    (matchesQuery.isPending && matchesQuery.fetchStatus !== 'idle') ||
    (isAuthenticated && myTeamQuery.isPending) ||
    (isAuthenticated && !!myTeamId && programsQuery.isPending);

  const error = tournamentQuery.isError || gameQuery.isError;

  if (isLoading) {
    return (
      <div className="flex justify-center py-24 text-sm text-gray-400"><Spinner>загрузка игры</Spinner></div>
    );
  }

  if (error || !game) {
    return (
      <ErrorState
        message={error ? 'Не удалось загрузить данные игры' : 'Игра не найдена'}
        onRetry={error ? () => { void tournamentQuery.refetch(); void gameQuery.refetch(); } : undefined}
      >
        <Link to={`/tournaments/${tournamentId}`} className="btn btn-secondary">
          Назад к турниру
        </Link>
      </ErrorState>
    );
  }

  return (
    <div>
      <PageHeader
        crumbs={[
          { label: 'турниры', to: '/tournaments' },
          { label: tournament?.name ?? 'турнир', to: `/tournaments/${tournamentId}` },
          { label: game.display_name },
        ]}
        title={
          <>
            <title>{`${game.display_name} — TJudge`}</title>
            <span aria-hidden="true" className={`mr-3 font-mono ${getGameConfig(game.name).textClass}`}>{getGameConfig(game.name).icon}</span>
            {game.display_name}
          </>
        }
      >
        <p className="text-gray-400 flex items-center gap-3 flex-wrap">
          <span>
            ID игры: <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{game.name}</code>
          </span>
          {gameStatus && (
            <AutoRoundCountdown status={gameStatus} tournamentActive={tournament?.status === 'active'} />
          )}
        </p>
      </PageHeader>

      {isAuthenticated && myTeam && tournamentId && (
        <ProgramSummary tournamentId={tournamentId} gameId={game.id} myTeam={myTeam} programs={programs} />
      )}

      <Tabs
        label="Разделы игры"
        items={[
          { id: 'rules', label: 'Правила' },
          { id: 'leaderboard', label: 'Рейтинг', count: leaderboard.length },
          { id: 'matches', label: 'Матчи' },
        ]}
        active={activeTab}
        onChange={setActiveTab}
      >
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Main Content Section */}
          <div className="lg:col-span-2 min-w-0">
            {activeTab === 'rules' && (
              <div className="card">
                <h2 className="text-lg font-semibold mb-4 text-gray-100">Правила игры</h2>
                {game.rules ? (
                  <div className="prose max-w-none prose-invert">
                    <div className="markdown-content text-gray-300">
                      <Markdown>{game.rules}</Markdown>
                    </div>
                  </div>
                ) : (
                  <p className="text-gray-400">Правила для этой игры не указаны.</p>
                )}
              </div>
            )}

            {activeTab === 'leaderboard' && (
              <div className="card">
                <h2 className="text-lg font-semibold mb-4 text-gray-100">Таблица рейтинга</h2>
                {leaderboard.length > 0 ? (
                  // relative: sr-only подпись в шапке иначе вылезает из прокрутки и растягивает страницу
                  <div className="relative overflow-x-auto">
                    <table className="w-full">
                      <thead>
                        <tr className="text-left text-sm text-gray-400 border-b border-gray-700">
                          <th className="pb-2 px-2">#</th>
                          <th className="pb-2 pr-4">Команда</th>
                          <th className="pb-2 pr-4 text-center" title="Сумма очков за сыгранные матчи раунда">Очки</th>
                          <th className="pb-2 pr-4 text-center">W</th>
                          <th className="pb-2 pr-4 text-center">L</th>
                          <th className="pb-2 pr-4 text-center">D</th>
                          <th className="pb-2 text-center">Игр</th>
                          <th className="pb-2 text-center"><span className="sr-only">График ELO</span></th>
                        </tr>
                      </thead>
                      <tbody>
                        {leaderboard.map((entry) => {
                          const mine = !!myTeamId && entry.team_id === myTeamId;
                          return (
                            <tr
                              key={entry.program_id}
                              ref={mine ? revealMine : undefined}
                              aria-current={mine ? 'true' : undefined}
                              className={`border-b border-gray-800 ${mine ? 'row-mine' : ''}`}
                            >
                              <td className="py-2 px-2 font-mono font-medium text-gray-200">{entry.rank}</td>
                              <td className="py-2 pr-4 text-gray-200">
                                {entry.team_name || entry.program_name}
                                {mine && <YouMark />}
                                {entry.team_name && (
                                  <div className="hidden font-mono text-xs text-gray-500 break-all sm:block">{entry.program_name}</div>
                                )}
                              </td>
                              <td className="py-2 pr-4 text-center font-mono font-medium text-gray-200">{entry.rating}</td>
                              <td className="py-2 pr-4 text-center font-mono text-green-400">{entry.wins}</td>
                              <td className="py-2 pr-4 text-center font-mono text-red-400">{entry.losses}</td>
                              <td className="py-2 pr-4 text-center font-mono text-gray-400">{entry.draws}</td>
                              <td className="py-2 text-center font-mono text-gray-200">{entry.total_games}</td>
                              <td className="py-2 text-center">
                                <button
                                  onClick={() => setChartProgram({ id: entry.program_id, name: entry.team_name || entry.program_name })}
                                  className="p-1.5 rounded-md text-gray-500 hover:text-primary-400 hover:bg-gray-800 transition-colors"
                                  title="График ELO"
                                  aria-label={`График ELO ${entry.team_name || entry.program_name}`}
                                >
                                  <ChartBarIcon className="w-4 h-4" />
                                </button>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <EmptyState command="рейтинг" hint="нет данных: загрузите программу и дождитесь результатов матчей" />
                )}

                {/* Head-to-head: кто кого бьёт */}
                {(headToHeadQuery.data?.length ?? 0) > 0 && (
                  <div className="mt-8 pt-6 border-t border-gray-800">
                    <h3 className="text-base font-semibold mb-4 text-gray-100">Личные встречи</h3>
                    <HeadToHeadMatrix cells={headToHeadQuery.data ?? []} myTeamId={myTeamId} />
                  </div>
                )}
              </div>
            )}

            {activeTab === 'matches' && (
              <div className="card">
                <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
                  <h2 id="game-matches" className="text-lg font-semibold text-gray-100">Результаты матчей</h2>
                  {myTeamId && (
                    <Segmented
                      label="Чьи матчи показать"
                      options={MATCH_VIEWS}
                      value={showAllMatches ? 'all' : 'mine'}
                      onChange={(v) => setMatchesParam('view', v === 'all' ? 'all' : null)}
                    />
                  )}
                </div>
                {onlyFailed && (
                  <p className="mb-4 flex flex-wrap items-center gap-3 font-mono text-sm text-gray-300">
                    <span><span aria-hidden="true" className="text-red-400">✕ </span>только матчи с ошибкой</span>
                    <button type="button" onClick={() => setMatchesParam('status', null)} className="btn btn-sm btn-secondary">
                      показать все
                    </button>
                  </p>
                )}
                {matches.length > 0 ? (
                  <MatchGroups matches={matches} me={{ teamId: myTeamId, teamName: myTeam?.name, programIds: myProgramIds }} />
                ) : (
                  <EmptyState
                    command="матчи"
                    hint={currentPage > 1 ? 'на этой странице матчей нет' : matchesTeam ? 'у вашей команды матчей пока нет' : 'матчи ещё не проводились'}
                  />
                )}
                {/* Pagination: и на опустевшей странице (раунд сбросили), чтобы было куда вернуться */}
                {(currentPage > 1 || hasNextPage) && (
                  <div className="flex items-center justify-center gap-2 mt-6 pt-4 border-t border-gray-700">
                    <button
                      onClick={() => setCurrentPage(currentPage - 1)}
                      disabled={currentPage === 1}
                      className="btn btn-secondary"
                    >
                      Назад
                    </button>
                    <span className="text-sm text-gray-400 px-4">
                      Страница {currentPage}
                    </span>
                    <button
                      onClick={() => setCurrentPage(currentPage + 1)}
                      disabled={!hasNextPage}
                      className="btn btn-secondary"
                    >
                      Вперёд
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Program Upload Section */}
          <div className="lg:col-span-1 min-w-0">
            {isAuthenticated && myTeam ? (
              <ProgramPanel
                tournament={tournament}
                gameId={game.id}
                gameStatus={gameStatus}
                gamesStatus={gamesStatusQuery.data ?? []}
                rounds={roundsQuery.data ?? []}
                myTeam={myTeam}
                programs={programs}
              />
            ) : (
              <div className="card">
                <h2 className="text-lg font-semibold mb-4 text-gray-100">Отправить программу</h2>
                {!isAuthenticated ? (
                  <p className="text-gray-400">
                    <Link to="/login" className="text-primary-400 hover:underline">
                      Войдите
                    </Link>{' '}
                    чтобы отправить программу.
                  </p>
                ) : (
                  <p className="text-gray-400">
                    <Link to={`/tournaments/${tournamentId}`} className="text-primary-400 hover:underline">
                      Присоединитесь к команде
                    </Link>{' '}
                    чтобы отправить программу.
                  </p>
                )}
              </div>
            )}
          </div>
        </div>
      </Tabs>
      {/* Rating chart modal */}
      <Modal
        open={chartProgram !== null}
        onClose={() => setChartProgram(null)}
        title={chartProgram ? `Динамика ELO — ${chartProgram.name}` : ''}
        maxWidth="max-w-2xl"
      >
        {ratingHistoryQuery.isPending ? (
          <div className="skeleton h-52 w-full" />
        ) : (
          <>
            <LineChart
              points={(ratingHistoryQuery.data ?? []).map((h) => ({
                value: h.new_rating,
                label: new Date(h.created_at).toLocaleString('ru-RU', {
                  day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit',
                }),
              }))}
            />
            {(ratingHistoryQuery.data?.length ?? 0) > 0 && (
              <p className="text-xs text-gray-500 mt-3">
                Последние {ratingHistoryQuery.data!.length} изменений ELO в этом турнире. ELO начинается с 1500
                и меняется после каждого матча с учётом силы соперника; это не сумма очков из таблицы
              </p>
            )}
          </>
        )}
      </Modal>
    </div>
  );
}

interface Me {
  teamId?: string;
  teamName?: string;
  /** Версии своей программы: запасной способ узнать свою сторону, если команд в ответе нет. */
  programIds: Set<string>;
}

// Сторона своей команды в матче; null - матч чужой.
function mySideOf(m: Match, me: Me): Side | null {
  if (me.teamId && m.team1_id === me.teamId) return 1;
  if (me.teamId && m.team2_id === me.teamId) return 2;
  if (me.programIds.has(m.program1_id)) return 1;
  if (me.programIds.has(m.program2_id)) return 2;
  return null;
}

const sideId = (m: Match, side: Side) => (side === 1 ? m.team1_id ?? m.program1_id : m.team2_id ?? m.program2_id);

// Матчи группируются по паре команд: пара играет в обеих ориентациях (AB и BA)
function MatchGroups({ matches, me }: { matches: Match[]; me: Me }) {
  const groups = new Map<string, Match[]>();
  for (const m of matches) {
    const key = [sideId(m, 1), sideId(m, 2)].sort().join('-');
    groups.set(key, [...(groups.get(key) ?? []), m]);
  }
  for (const group of groups.values()) {
    group.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
  }

  return (
    <div className="space-y-4">
      {[...groups].map(([key, group]) => (
        <MatchGroupCard key={key} matches={group} me={me} />
      ))}
    </div>
  );
}

// Карточка пары: слева своя команда (от первого лица), у чужой пары - сторона первого матча.
// Счёт, цвета и итог считаются для левой стороны в каждом матче, в какой бы роли она ни играла.
function MatchGroupCard({ matches, me }: { matches: Match[]; me: Me }) {
  const [activeIndex, setActiveIndex] = useState(0);
  const activeMatch = matches[activeIndex];
  const mine = matches.some((m) => mySideOf(m, me) !== null);
  const leftId = sideId(matches[0], 1);
  const leftSide = (m: Match): Side => (mine ? mySideOf(m, me) ?? 1 : sideId(m, 1) === leftId ? 1 : 2);
  const nameOf = (m: Match, side: Side) =>
    (side === 1 ? m.team1_name : m.team2_name) ??
    (mine && side === leftSide(m) ? me.teamName : undefined) ??
    `программа ${(side === 1 ? m.program1_id : m.program2_id).slice(0, 8)}`;
  const other = (side: Side): Side => (side === 1 ? 2 : 1);
  const scoreOf = (m: Match, side: Side) => (side === 1 ? m.score1 : m.score2);

  const first = matches[0];
  const leftName = nameOf(first, leftSide(first));
  const rightName = nameOf(first, other(leftSide(first)));
  const { leftTotal, rightTotal, wins, losses, draws, finished } = pairTotals(matches, leftSide);
  // цвета от первого лица (у чужой пары - от левой стороны): победа зелёная, поражение красное
  const tone = mine ? (wins > losses ? 'text-green-400' : wins < losses ? 'text-red-400' : 'text-gray-100') : 'text-gray-100';

  const resultOf = (m: Match) => {
    const ls = leftSide(m);
    if (m.winner === ls) return mine ? 'победа' : `победа «${leftName}»`;
    if (m.winner === other(ls)) return mine ? 'поражение' : `победа «${rightName}»`;
    if (m.status === 'completed') return 'ничья';
    return undefined;
  };
  const roleOf = (m: Match) => (mine ? `вы — игрок ${leftSide(m)}` : `«${leftName}» — игрок ${leftSide(m)}`);
  const cellColor = (m: Match) => {
    const ls = leftSide(m);
    if (m.status === 'running') return 'bg-blue-500 animate-pulse';
    if (m.status === 'pending' || m.status === 'cancelled') return 'bg-line';
    if (m.winner === ls) return 'bg-green-600';
    if (m.winner === other(ls)) return 'bg-red-600';
    return m.status === 'failed' ? 'bg-red-800' : 'bg-gray-600';
  };

  return (
    <div className={`bg-gray-800/50 rounded-lg p-4 border border-gray-700 ${mine ? 'row-mine' : ''}`}>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-4 gap-2">
        <div className="flex items-center gap-4 min-w-0">
          {/* своё имя участник знает: слева только «вы», место - названию соперника */}
          <div className={mine ? 'shrink-0' : 'min-w-0'}>
            {mine ? (
              <p className="text-sm font-medium">
                <YouMark className="text-sm" />
              </p>
            ) : (
              <p className="text-sm font-medium text-gray-300 truncate">{leftName}</p>
            )}
            <p className={`text-2xl font-bold font-mono tabular-nums ${tone}`}>{leftTotal}</p>
          </div>
          <span aria-hidden="true" className="text-lg text-gray-500">:</span>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium text-gray-300 truncate">{rightName}</p>
            <p className="text-2xl font-bold font-mono tabular-nums text-gray-100">{rightTotal}</p>
          </div>
        </div>

        <p className="font-mono text-sm text-gray-400">
          {mine ? (
            <>
              <span className="text-green-400">победы {wins}</span> · ничьи {draws} ·{' '}
              <span className="text-red-400">поражения {losses}</span>
            </>
          ) : (
            <>победы {wins}:{losses} · ничьи {draws}</>
          )}
          <span className="text-gray-500"> [{finished}/{matches.length}]</span>
        </p>
      </div>

      {/* Матчи пары: обе ориентации, AB и BA */}
      <div className="mb-3 flex flex-wrap gap-1.5" role="group" aria-label={`Матчи: ${leftName} против ${rightName}`}>
        {matches.map((match, index) => {
          const result = resultOf(match);
          return (
            <button
              key={match.id}
              type="button"
              onClick={() => setActiveIndex(index)}
              aria-pressed={activeIndex === index}
              aria-label={`Матч ${index + 1}: ${roleOf(match)}${result ? `, ${result}` : ''}`}
              title={`Матч ${index + 1}: ${roleOf(match)}${result ? `, ${result}` : ''}`}
              className={`w-8 h-8 rounded-lg text-xs font-medium transition-all ${
                activeIndex === index ? 'ring-2 ring-primary-500 ring-offset-1 ring-offset-gray-800' : 'hover:scale-105'
              }`}
            >
              <div className={`w-full h-full rounded-lg flex items-center justify-center text-white ${cellColor(match)}`}>
                {index + 1}
              </div>
            </button>
          );
        })}
      </div>

      {activeMatch && (
        <div className="bg-gray-800 rounded-lg p-3 border border-line">
          <div className="flex items-center justify-between gap-2 mb-2">
            <span className="text-sm font-medium text-gray-300">
              Матч {activeIndex + 1} · {roleOf(activeMatch)}
            </span>
            <StatusLabel entity="match" status={activeMatch.status} />
          </div>

          <div className="flex items-center justify-center gap-4 py-2 font-mono tabular-nums">
            {(() => {
              const ls = leftSide(activeMatch);
              const leftWon = activeMatch.winner === ls;
              const rightWon = activeMatch.winner === other(ls);
              return (
                <>
                  <span className={`text-xl font-bold ${leftWon ? 'text-green-400' : rightWon && mine ? 'text-red-400' : 'text-gray-300'}`}>
                    {scoreOf(activeMatch, ls) ?? '-'}
                  </span>
                  <span className="text-gray-400">:</span>
                  <span className={`text-xl font-bold ${rightWon && !mine ? 'text-green-400' : 'text-gray-300'}`}>
                    {scoreOf(activeMatch, other(ls)) ?? '-'}
                  </span>
                </>
              );
            })()}
          </div>

          {activeMatch.status === 'completed' && (
            <p className="text-center text-sm text-gray-400">{resultOf(activeMatch)}</p>
          )}

          {activeMatch.status === 'failed' && (
            <div className="mt-2">
              <MatchError
                match={activeMatch}
                mySide={mine ? leftSide(activeMatch) : null}
                names={[nameOf(activeMatch, 1), nameOf(activeMatch, 2)]}
              />
            </div>
          )}

          <p className="text-xs text-gray-500 mt-2 text-center">
            {new Date(activeMatch.created_at).toLocaleString('ru-RU')}
          </p>
        </div>
      )}
    </div>
  );
}
