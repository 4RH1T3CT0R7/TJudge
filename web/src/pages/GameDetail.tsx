import { useState, useRef, useMemo } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
import axios from 'axios';
import { useQuery, useQueryClient, keepPreviousData } from '@tanstack/react-query';
import api from '../api/client';
import { queryKeys } from '../api/queryKeys';
import {
  useTournament,
  useGame,
  useTournamentGamesStatus,
  useGameLeaderboard,
  useMyTeam,
} from '../hooks/queries';
import { useTournamentLive } from '../hooks/useTournamentLive';
import { useAuthStore } from '../store/authStore';
import { useToastStore } from '../store/toastStore';
import { SpaceInvader } from '../components/SpaceInvader';
import { Modal } from '../components/ui/Modal';
import { Tabs } from '../components/ui/Tabs';
import { StatusLabel } from '../components/ui/StatusLabel';
import { Spinner } from '../components/ui/Spinner';
import { TerminalOutput } from '../components/ui/TerminalOutput';
import { Segmented } from '../components/ui/Segmented';
import { MatchError } from '../components/tournament/MatchError';
import type { Side } from '../utils/explainError';
import { EmptyState } from '../components/ui/EmptyState';
import { ErrorState } from '../components/ui/ErrorState';
import { PageHeader } from '../components/ui/PageHeader';
import { LineChart } from '../components/ui/LineChart';
import { Markdown } from '../components/ui/Markdown';
import { YouMark } from '../components/ui/YouMark';
import { useRevealOnMobile } from '../hooks/useRevealOnMobile';
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
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useTabParam(TAB_IDS, 'rules');

  const matchesPerPage = 20;

  // Upload state
  const [isUploading, setIsUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  // id загруженной версии: сообщение об успехе снимается, если её сборка упала
  const [uploadedId, setUploadedId] = useState<string | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [uploadInvaderBubble, setUploadInvaderBubble] = useState<string | null>('// жду код...');
  const [uploadInvaderShake, setUploadInvaderShake] = useState(false);
  const [uploadInvaderJump, setUploadInvaderJump] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dropZoneRef = useRef<HTMLDivElement>(null);

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

  // График рейтинга: история подгружается лениво при открытии модалки.
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
  const pageData = matchesQuery.data ?? [];
  const hasNextPage = pageData.length > matchesPerPage;
  const matches = hasNextPage ? pageData.slice(0, matchesPerPage) : pageData;

  // Версии команды по игре, кто бы из участников их ни загрузил: в матчах
  // играет последняя версия команды, а не последняя своя. Поллинг каждые 10с,
  // пока какая-то версия компилируется (бейдж статуса обновится сам)
  const revealMine = useRevealOnMobile<HTMLTableRowElement>();
  const programsQuery = useQuery({
    queryKey: queryKeys.programVersions(myTeamId ?? '', gameId ?? ''),
    queryFn: () => api.getProgramVersions(myTeamId!, gameId!),
    enabled: isAuthenticated && !!myTeamId && !!gameId,
    refetchInterval: (query) =>
      query.state.data?.some((p) => p.status === 'compiling') ? 10000 : false,
  });
  const programs = useMemo(() => programsQuery.data ?? [], [programsQuery.data]);
  const myProgramIds = useMemo(() => new Set(programs.map((p) => p.id)), [programs]);
  const currentProgram = useMemo(
    () =>
      programs.length > 0
        ? programs.reduce((a, b) => (a.version > b.version ? a : b))
        : null,
    [programs]
  );

  const isLoading =
    tournamentQuery.isPending ||
    gameQuery.isPending ||
    gamesStatusQuery.isPending ||
    leaderboardQuery.isPending ||
    (matchesQuery.isPending && matchesQuery.fetchStatus !== 'idle') ||
    (isAuthenticated && myTeamQuery.isPending) ||
    (isAuthenticated && !!myTeamId && programsQuery.isPending);

  const error = tournamentQuery.isError || gameQuery.isError;

  const canUpload = tournament?.status === 'active' && !gameStatus?.round_completed && !isUploading;

  const handleFileSelect = () => {
    fileInputRef.current?.click();
  };

  // Как detectLanguage в internal/handlers/program.go
  const supportedExtensions = ['.py', '.cpp', '.cc', '.cxx', '.c', '.go', '.rs', '.java', '.js', '.rb', '.php', '.lua'];

  const isValidFile = (file: File) => {
    const ext = '.' + file.name.split('.').pop()?.toLowerCase();
    return supportedExtensions.includes(ext);
  };

  // Process uploaded file (used by both input and drag-drop)
  const processFile = async (file: File) => {
    setUploadedId(null);
    if (!tournamentId || !gameId || !myTeam) {
      setUploadError('Не удалось загрузить: отсутствуют данные команды');
      return;
    }

    if (!isValidFile(file)) {
      setUploadError(`Неподдерживаемый формат файла. Используйте: ${supportedExtensions.join(', ')}`);
      return;
    }

    setIsUploading(true);
    setUploadError(null);
    setUploadInvaderBubble('// загружаю...');

    try {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('team_id', myTeam.id);
      formData.append('tournament_id', tournamentId);
      formData.append('game_id', gameId);
      formData.append('name', file.name);

      const program = await api.uploadProgram(formData);
      // Кэш программ обновится сам - текущая версия и список пересчитаются
      queryClient.invalidateQueries({ queryKey: queryKeys.programs });

      // Check for syntax errors in uploaded program
      if (program.error_message) {
        // Program uploaded but has syntax error - show warning
        setUploadError(`Программа загружена, но обнаружена ошибка синтаксиса:\n${program.error_message}`);
      } else {
        setUploadedId(program.id);
        setUploadInvaderBubble('{ загружено: true }');
        setUploadInvaderJump(true);
        setTimeout(() => setUploadInvaderJump(false), 100);
        // сообщение об успехе держится до следующей загрузки или провала сборки, реплика маскота - 3 с
        setTimeout(() => setUploadInvaderBubble('// жду код...'), 3000);
      }

      // Clear file input
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }
    } catch (err: unknown) {
      console.error('Upload failed:', err);
      setUploadInvaderBubble('// ошибка!');
      setUploadInvaderShake(true);
      setTimeout(() => setUploadInvaderShake(false), 100);
      setTimeout(() => setUploadInvaderBubble('// жду код...'), 3000);
      // Extract error message from API response
      if (axios.isAxiosError(err)) {
        setUploadError(err.response?.data?.error || err.response?.data?.message || 'Не удалось загрузить программу');
      } else {
        setUploadError('Не удалось загрузить программу');
      }
    } finally {
      setIsUploading(false);
    }
  };

  // Drag and drop handlers
  const handleDragEnter = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(true);
    setUploadInvaderBubble('// давай сюда!');
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    // Only set dragging to false if we're leaving the drop zone entirely
    if (dropZoneRef.current && !dropZoneRef.current.contains(e.relatedTarget as Node)) {
      setIsDragging(false);
      setUploadInvaderBubble('// жду код...');
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    setUploadedId(null);

    if (tournament?.status === 'completed') {
      setUploadError('Турнир завершён, загрузка программ закрыта');
      return;
    }

    if (gameStatus?.round_completed) {
      setUploadError('Раунд для этой игры завершён, загрузка новых версий закрыта');
      return;
    }

    const files = e.dataTransfer.files;
    if (files && files.length > 0) {
      processFile(files[0]);
    }
  };

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;
    processFile(file);
  };

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
                  <div className="overflow-x-auto">
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
                          <th className="pb-2 text-center" aria-label="График рейтинга"></th>
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
                                  <div className="font-mono text-xs text-gray-500 break-all">{entry.program_name}</div>
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
                                  title="График рейтинга"
                                  aria-label={`График рейтинга ${entry.team_name || entry.program_name}`}
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
                  <h2 className="text-lg font-semibold text-gray-100">Результаты матчей</h2>
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
              <div className="card">
                <h2 className="text-lg font-semibold mb-4 text-gray-100">Ваша программа</h2>

                {/* Show warning if tournament is completed or not accepting submissions */}
                {tournament?.status === 'completed' && (
                  <div className="mb-4 p-3 bg-gray-800 rounded-lg border border-gray-700">
                    <div className="flex items-center gap-2 text-gray-400">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
                      </svg>
                      <span className="text-sm font-medium">Турнир завершён</span>
                    </div>
                    <p className="text-xs text-gray-500 mt-1">
                      Загрузка программ больше не доступна
                    </p>
                  </div>
                )}

                {tournament?.status === 'pending' && (
                  <div className="mb-4 p-3 bg-yellow-900/30 rounded-lg border border-yellow-700">
                    <div className="flex items-center gap-2 text-yellow-300">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                      </svg>
                      <span className="text-sm font-medium">Турнир ещё не начался</span>
                    </div>
                    <p className="text-xs text-yellow-400 mt-1">
                      Загрузка программ будет доступна после начала турнира
                    </p>
                  </div>
                )}

                {gameStatus?.round_completed && (
                  <div className="mb-4 p-3 bg-orange-900/30 rounded-lg border border-orange-700">
                    <div className="flex items-center gap-2 text-orange-300">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12c0 1.268-.63 2.39-1.593 3.068a3.745 3.745 0 0 1-1.043 3.296 3.745 3.745 0 0 1-3.296 1.043A3.745 3.745 0 0 1 12 21c-1.268 0-2.39-.63-3.068-1.593a3.746 3.746 0 0 1-3.296-1.043 3.745 3.745 0 0 1-1.043-3.296A3.745 3.745 0 0 1 3 12c0-1.268.63-2.39 1.593-3.068a3.745 3.745 0 0 1 1.043-3.296 3.746 3.746 0 0 1 3.296-1.043A3.746 3.746 0 0 1 12 3c1.268 0 2.39.63 3.068 1.593a3.746 3.746 0 0 1 3.296 1.043 3.746 3.746 0 0 1 1.043 3.296A3.745 3.745 0 0 1 21 12Z" />
                      </svg>
                      <span className="text-sm font-medium">Раунд завершён</span>
                    </div>
                    <p className="text-xs text-orange-400 mt-1">
                      Раунд для этой игры завершён, загрузка новых версий закрыта
                    </p>
                    {gameStatus.round_completed_at && (
                      <p className="text-xs text-orange-500 mt-1">
                        Завершён: {new Date(gameStatus.round_completed_at).toLocaleString('ru-RU')}
                      </p>
                    )}
                  </div>
                )}

                {/* Current Program */}
                {currentProgram && (
                  <div className="mb-4 p-3 bg-gray-800 rounded-lg">
                    <div className="flex flex-wrap justify-between items-start gap-2 mb-2">
                      <p className="min-w-0 break-all font-medium text-gray-200">{currentProgram.name}</p>
                      <div className="flex items-center gap-2">
                        <StatusLabel entity="program" status={currentProgram.status} />
                        <span className="text-xs bg-primary-900/50 text-primary-300 px-2 py-0.5 rounded">
                          v{currentProgram.version}
                        </span>
                      </div>
                    </div>
                    <p className="text-sm text-gray-400">
                      Загружена: {new Date(currentProgram.created_at).toLocaleString('ru-RU')}
                    </p>
                    {currentProgram.error_message && (
                      <div className="mt-2">
                        <TerminalOutput label="вывод компилятора" text={currentProgram.error_message} />
                      </div>
                    )}
                    <button
                      onClick={async () => {
                        try {
                          const blob = await api.downloadProgram(currentProgram.id);
                          const url = window.URL.createObjectURL(blob);
                          const a = document.createElement('a');
                          a.href = url;
                          a.download = currentProgram.name || 'program';
                          document.body.appendChild(a);
                          a.click();
                          window.URL.revokeObjectURL(url);
                          document.body.removeChild(a);
                        } catch (err) {
                          console.error('Download failed:', err);
                          useToastStore.getState().addToast('Не удалось скачать программу', 'error');
                        }
                      }}
                      className="btn btn-secondary w-full mt-2"
                    >
                      Скачать программу
                    </button>
                  </div>
                )}

                {/* Upload Form with Drag & Drop */}
                <div className="space-y-3">
                  {/* Upload invader */}
                  <div className="flex justify-center pt-6">
                    <SpaceInvader
                      size="sm"
                      speechBubble={uploadInvaderBubble}
                      shake={uploadInvaderShake}
                      jump={uploadInvaderJump}
                      eyeOverride={isDragging ? 'wide' : null}
                    />
                  </div>

                  <input
                    type="file"
                    ref={fileInputRef}
                    onChange={handleFileUpload}
                    className="hidden"
                    accept={supportedExtensions.join(',')}
                    aria-label="Загрузить файл программы"
                  />

                  {/* Drop Zone */}
                  <div
                    ref={dropZoneRef}
                    role="button"
                    tabIndex={canUpload ? 0 : -1}
                    aria-label="Загрузить файл программы"
                    aria-disabled={!canUpload || undefined}
                    onKeyDown={(e) => {
                      if (canUpload && (e.key === 'Enter' || e.key === ' ')) {
                        e.preventDefault();
                        handleFileSelect();
                      }
                    }}
                    onDragEnter={handleDragEnter}
                    onDragLeave={handleDragLeave}
                    onDragOver={handleDragOver}
                    onDrop={handleDrop}
                    onClick={canUpload ? handleFileSelect : undefined}
                    className={`
                      relative border-2 border-dashed rounded-lg p-6 text-center transition-all cursor-pointer
                      ${!canUpload ? 'cursor-not-allowed opacity-50' : ''}
                      ${isDragging
                        ? 'border-primary-500 bg-primary-900/20'
                        : 'border-line hover:border-primary-500 hover:bg-gray-800/50'
                      }
                    `}
                  >
                    {isDragging ? (
                      <div className="flex flex-col items-center gap-2">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-10 h-10 text-primary-500">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
                        </svg>
                        <p className="text-sm font-medium text-primary-400">
                          Отпустите файл для загрузки
                        </p>
                      </div>
                    ) : (
                      <div className="flex flex-col items-center gap-2">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-10 h-10 text-gray-500">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m6.75 12-3-3m0 0-3 3m3-3v6m-1.5-15H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
                        </svg>
                        <div>
                          <p className="text-sm font-medium text-gray-300">
                            {isUploading ? 'Загрузка...' : tournament?.status === 'completed' ? 'Загрузка закрыта' : gameStatus?.round_completed ? 'Раунд завершён' : 'Перетащите файл сюда'}
                          </p>
                          {tournament?.status !== 'completed' && !gameStatus?.round_completed && !isUploading && (
                            <p className="text-xs text-gray-400 mt-1">
                              или <span className="text-primary-400 underline">выберите файл</span>
                            </p>
                          )}
                        </div>
                      </div>
                    )}

                    {isUploading && (
                      <div className="absolute inset-0 bg-gray-900/50 rounded-lg flex items-center justify-center">
                        <Spinner className="text-lg" />
                      </div>
                    )}
                  </div>

                  {uploadedId && programs.find((p) => p.id === uploadedId)?.status !== 'failed' && (
                    <div role="status" className="p-2 bg-green-900/30 border border-green-700 rounded text-sm text-green-300 flex items-center gap-2">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                      </svg>
                      Программа успешно загружена!
                    </div>
                  )}

                  {uploadError && (
                    <div role="alert" className="p-2 bg-red-900/30 border border-red-700 rounded text-sm text-red-300 flex items-center gap-2">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
                      </svg>
                      {uploadError}
                    </div>
                  )}

                  <p className="text-xs text-gray-400 text-center">
                    Поддерживаемые форматы: {supportedExtensions.join(', ')}
                  </p>
                </div>

                {/* Previous Versions */}
                {programs.length > 1 && (
                  <div className="mt-6">
                    <h3 className="font-medium mb-2 text-gray-100">Предыдущие версии</h3>
                    <div className="space-y-2">
                      {programs
                        .filter((p) => p.id !== currentProgram?.id)
                        .sort((a, b) => b.version - a.version)
                        .map((program) => (
                          <div
                            key={program.id}
                            className="flex justify-between items-center text-sm p-2 bg-gray-800 rounded"
                          >
                            <div className="flex flex-col">
                              <div className="flex items-center gap-2">
                                <span className="text-gray-100">v{program.version}</span>
                                <StatusLabel
                                  entity="program"
                                  status={program.status}
                                  title={program.status === 'failed' ? program.error_message || undefined : undefined}
                                />
                              </div>
                              <span className="text-xs text-gray-400">
                                {new Date(program.created_at).toLocaleDateString('ru-RU')}
                              </span>
                            </div>
                            <button
                              onClick={async () => {
                                try {
                                  const blob = await api.downloadProgram(program.id);
                                  const url = window.URL.createObjectURL(blob);
                                  const a = document.createElement('a');
                                  a.href = url;
                                  a.download = program.name || `program_v${program.version}`;
                                  document.body.appendChild(a);
                                  a.click();
                                  window.URL.revokeObjectURL(url);
                                  document.body.removeChild(a);
                                } catch (err) {
                                  console.error('Download failed:', err);
                                  useToastStore.getState().addToast('Не удалось скачать программу', 'error');
                                }
                              }}
                              className="text-primary-400 hover:text-primary-300 text-xs font-medium"
                            >
                              Скачать
                            </button>
                          </div>
                        ))}
                    </div>
                  </div>
                )}
              </div>
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
        title={chartProgram ? `Динамика рейтинга — ${chartProgram.name}` : ''}
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
                Последние {ratingHistoryQuery.data!.length} изменений рейтинга в этом турнире
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
  let leftTotal = 0, rightTotal = 0, wins = 0, losses = 0, draws = 0, finished = 0;
  for (const m of matches) {
    const ls = leftSide(m);
    leftTotal += scoreOf(m, ls) ?? 0;
    rightTotal += scoreOf(m, other(ls)) ?? 0;
    if (m.status === 'completed' || m.status === 'failed') finished++;
    if (m.winner === ls) wins++;
    else if (m.winner === other(ls)) losses++;
    else if (m.status === 'completed' && m.winner === 0) draws++;
  }
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
          <div className="min-w-0">
            <p className="text-sm font-medium text-gray-300 truncate">
              {leftName}
              {mine && <YouMark />}
            </p>
            <p className={`text-2xl font-bold font-mono tabular-nums ${tone}`}>{leftTotal}</p>
          </div>
          <span aria-hidden="true" className="text-lg text-gray-500">:</span>
          <div className="min-w-0">
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
