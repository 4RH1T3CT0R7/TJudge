import { useState, useRef, useMemo } from 'react';
import { useParams, Link } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
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
import { useAuthStore } from '../store/authStore';
import { useToastStore } from '../store/toastStore';
import { SpaceInvader } from '../components/SpaceInvader';
import { Modal } from '../components/ui/Modal';
import { LineChart } from '../components/ui/LineChart';
import { HeadToHeadMatrix } from '../components/tournament/HeadToHeadMatrix';
import { AutoRoundCountdown } from '../components/tournament/AutoRoundCountdown';
import { ChartBarIcon } from '../components/icons';
import { getGameConfig } from '../utils/gameConfig';
import type { Program, Match } from '../types';

const remarkPlugins = [remarkGfm];

export function GameDetail() {
  const { tournamentId, gameId } = useParams<{ tournamentId: string; gameId: string }>();
  const { isAuthenticated } = useAuthStore();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<'rules' | 'leaderboard' | 'matches'>('rules');

  // Pagination state for matches
  const [currentPage, setCurrentPage] = useState(1);
  const matchesPerPage = 20;

  // Upload state
  const [isUploading, setIsUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploadSuccess, setUploadSuccess] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [uploadInvaderBubble, setUploadInvaderBubble] = useState<string | null>('// жду код...');
  const [uploadInvaderShake, setUploadInvaderShake] = useState(false);
  const [uploadInvaderJump, setUploadInvaderJump] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const dropZoneRef = useRef<HTMLDivElement>(null);

  // Базовые данные страницы
  const tournamentQuery = useTournament(tournamentId ?? '');
  const gameQuery = useGame(gameId ?? '');
  const gamesStatusQuery = useTournamentGamesStatus(tournamentId ?? '');
  const leaderboardQuery = useGameLeaderboard(tournamentId ?? '', gameId ?? '');
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

  // Матчи с пагинацией: ключ включает страницу, предыдущая страница
  // остаётся на экране, пока грузится новая
  const matchesQuery = useQuery({
    queryKey: [...queryKeys.gameMatches(tournamentId ?? '', gameId ?? ''), currentPage] as const,
    queryFn: () =>
      api.getGameMatches(
        tournamentId ?? '',
        gameId ?? '',
        undefined,
        matchesPerPage,
        (currentPage - 1) * matchesPerPage
      ),
    enabled: !!tournamentId && !!gameId,
    placeholderData: keepPreviousData,
  });
  const matches = matchesQuery.data ?? [];
  const totalMatches = matches.length;

  // Программы команды: поллинг каждые 10с, пока какая-то версия
  // компилируется (бейдж статуса обновится сам)
  const myTeamId = myTeam?.id;
  const programsQuery = useQuery({
    queryKey: queryKeys.programs,
    queryFn: () => api.getPrograms(),
    enabled: isAuthenticated && !!myTeamId,
    refetchInterval: (query) => {
      const hasCompiling = query.state.data?.some(
        (p) => p.team_id === myTeamId && p.game_id === gameId && p.status === 'compiling'
      );
      return hasCompiling ? 10000 : false;
    },
  });
  const programs = useMemo(
    () =>
      (programsQuery.data ?? []).filter(
        (p) => p.team_id === myTeamId && p.game_id === gameId
      ),
    [programsQuery.data, myTeamId, gameId]
  );
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
    matchesQuery.isPending ||
    (isAuthenticated && myTeamQuery.isPending) ||
    (isAuthenticated && !!myTeamId && programsQuery.isPending);

  const error = tournamentQuery.isError || gameQuery.isError;

  const canUpload = tournament?.status === 'active' && !gameStatus?.round_completed && !isUploading;

  const handleFileSelect = () => {
    fileInputRef.current?.click();
  };

  // Supported file extensions
  const supportedExtensions = ['.py', '.cpp', '.c', '.go', '.rs', '.java'];

  const isValidFile = (file: File) => {
    const ext = '.' + file.name.split('.').pop()?.toLowerCase();
    return supportedExtensions.includes(ext);
  };

  // Process uploaded file (used by both input and drag-drop)
  const processFile = async (file: File) => {
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
    setUploadSuccess(false);
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
        setUploadError(`⚠️ Программа загружена, но обнаружена ошибка синтаксиса:\n${program.error_message}`);
      } else {
        setUploadSuccess(true);
        setUploadInvaderBubble('{ загружено: true }');
        setUploadInvaderJump(true);
        setTimeout(() => setUploadInvaderJump(false), 100);
        // Hide success message after 3 seconds
        setTimeout(() => {
          setUploadSuccess(false);
          setUploadInvaderBubble('// жду код...');
        }, 3000);
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
      <div className="text-center py-12">
        <SpaceInvader size="sm" />
        <p className="text-gray-500 mt-3 font-mono text-sm">// загрузка...</p>
      </div>
    );
  }

  if (error || !game) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">{error ? 'Не удалось загрузить данные игры' : 'Игра не найдена'}</p>
        <Link to={`/tournaments/${tournamentId}`} className="btn btn-secondary mt-4">
          Назад к турниру
        </Link>
      </div>
    );
  }

  return (
    <div>
      {/* Breadcrumb */}
      <nav className="mb-4 text-sm">
        <Link to="/tournaments" className="text-gray-400 hover:text-gray-300">
          Турниры
        </Link>
        <span className="mx-2 text-gray-600">/</span>
        <Link to={`/tournaments/${tournamentId}`} className="text-gray-400 hover:text-gray-300">
          Турнир
        </Link>
        <span className="mx-2 text-gray-600">/</span>
        <span className="text-gray-200">{game.display_name}</span>
      </nav>

      {/* Header */}
      <div className="mb-6">
        <div className="flex items-center gap-4">
          <div className={`w-14 h-14 ${getGameConfig(game.name).bgClass} rounded-xl flex items-center justify-center text-3xl shadow-lg`}>
            {getGameConfig(game.name).icon}
          </div>
          <div>
            <h1 className={`text-2xl font-bold mb-1 text-gray-100`}>{game.display_name}</h1>
            <p className="text-gray-400 flex items-center gap-3 flex-wrap">
              <span>
                ID игры: <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{game.name}</code>
              </span>
              {gameStatus && (
                <AutoRoundCountdown
                  enabled={gameStatus.auto_round_enabled}
                  intervalSeconds={gameStatus.auto_round_interval_seconds}
                  lastRunAt={gameStatus.auto_round_last_run_at}
                />
              )}
            </p>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-700 mb-6">
        <nav className="-mb-px flex space-x-8">
          <button
            onClick={() => setActiveTab('rules')}
            className={`py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === 'rules'
                ? 'border-primary-500 text-primary-400'
                : 'border-transparent text-gray-400 hover:text-gray-300 hover:border-gray-600'
            }`}
          >
            Правила
          </button>
          <button
            onClick={() => setActiveTab('leaderboard')}
            className={`py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === 'leaderboard'
                ? 'border-primary-500 text-primary-400'
                : 'border-transparent text-gray-400 hover:text-gray-300 hover:border-gray-600'
            }`}
          >
            Рейтинг ({leaderboard.length})
          </button>
          <button
            onClick={() => setActiveTab('matches')}
            className={`py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === 'matches'
                ? 'border-primary-500 text-primary-400'
                : 'border-transparent text-gray-400 hover:text-gray-300 hover:border-gray-600'
            }`}
          >
            Матчи ({matches.length})
          </button>
        </nav>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main Content Section */}
        <div className="lg:col-span-2">
          {activeTab === 'rules' && (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4 text-gray-100">Правила игры</h2>
              {game.rules ? (
                <div className="prose max-w-none prose-invert">
                  <div className="markdown-content text-gray-300">
                    <ReactMarkdown remarkPlugins={remarkPlugins}>{game.rules}</ReactMarkdown>
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
                        <th className="pb-2 pr-4">#</th>
                        <th className="pb-2 pr-4">Программа</th>
                        <th className="pb-2 pr-4 text-center">Рейтинг</th>
                        <th className="pb-2 pr-4 text-center">W</th>
                        <th className="pb-2 pr-4 text-center">L</th>
                        <th className="pb-2 pr-4 text-center">D</th>
                        <th className="pb-2 text-center">Игр</th>
                        <th className="pb-2 text-center" aria-label="График рейтинга"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {leaderboard.map((entry) => (
                        <tr key={entry.program_id} className="border-b border-gray-800">
                          <td className="py-2 pr-4 font-medium text-gray-200">{entry.rank}</td>
                          <td className="py-2 pr-4 text-gray-200">{entry.program_name}</td>
                          <td className="py-2 pr-4 text-center font-medium text-gray-200">{entry.rating}</td>
                          <td className="py-2 pr-4 text-center text-green-400">{entry.wins}</td>
                          <td className="py-2 pr-4 text-center text-red-400">{entry.losses}</td>
                          <td className="py-2 pr-4 text-center text-gray-400">{entry.draws}</td>
                          <td className="py-2 text-center text-gray-200">{entry.total_games}</td>
                          <td className="py-2 text-center">
                            <button
                              onClick={() => setChartProgram({ id: entry.program_id, name: entry.program_name })}
                              className="p-1.5 rounded-md text-gray-500 hover:text-primary-400 hover:bg-gray-800 transition-colors"
                              title="График рейтинга"
                              aria-label={`График рейтинга ${entry.program_name}`}
                            >
                              <ChartBarIcon className="w-4 h-4" />
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="text-gray-400">Нет данных рейтинга. Загрузите программу и дождитесь результатов матчей.</p>
              )}

              {/* Head-to-head: кто кого бьёт */}
              {(headToHeadQuery.data?.length ?? 0) > 0 && (
                <div className="mt-8 pt-6 border-t border-gray-800">
                  <h3 className="text-base font-semibold mb-4 text-gray-100">Личные встречи</h3>
                  <HeadToHeadMatrix cells={headToHeadQuery.data ?? []} />
                </div>
              )}
            </div>
          )}

          {activeTab === 'matches' && (
            <div className="card">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-lg font-semibold text-gray-100">Результаты матчей</h2>
                {matches.length > 0 && (
                  <span className="text-sm text-gray-400">
                    Всего: {totalMatches}
                  </span>
                )}
              </div>
              {matches.length > 0 ? (
                <>
                  <MatchGroups matches={matches} />
                  {/* Pagination */}
                  {totalMatches > matchesPerPage && (
                    <div className="flex items-center justify-center gap-2 mt-6 pt-4 border-t border-gray-700">
                      <button
                        onClick={() => setCurrentPage((p) => p - 1)}
                        disabled={currentPage === 1}
                        className="btn btn-secondary text-sm disabled:opacity-50"
                      >
                        Назад
                      </button>
                      <span className="text-sm text-gray-400 px-4">
                        Страница {currentPage} из {Math.ceil(totalMatches / matchesPerPage)}
                      </span>
                      <button
                        onClick={() => setCurrentPage((p) => p + 1)}
                        disabled={currentPage >= Math.ceil(totalMatches / matchesPerPage)}
                        className="btn btn-secondary text-sm disabled:opacity-50"
                      >
                        Вперёд
                      </button>
                    </div>
                  )}
                </>
              ) : (
                <p className="text-gray-400">Матчи ещё не проводились.</p>
              )}
            </div>
          )}
        </div>

        {/* Program Upload Section */}
        <div className="lg:col-span-1">
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
                  <div className="flex justify-between items-start mb-2">
                    <p className="font-medium text-gray-200">{currentProgram.name}</p>
                    <div className="flex items-center gap-2">
                      <ProgramStatusBadge program={currentProgram} />
                      <span className="text-xs bg-primary-900/50 text-primary-300 px-2 py-0.5 rounded">
                        v{currentProgram.version}
                      </span>
                    </div>
                  </div>
                  <p className="text-sm text-gray-400">
                    Загружена: {new Date(currentProgram.created_at).toLocaleString('ru-RU')}
                  </p>
                  {currentProgram.error_message && (
                    <div className="mt-2 p-2 bg-red-900/30 border border-red-700 rounded text-sm text-red-300">
                      <strong>Ошибка:</strong> {currentProgram.error_message}
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
                    className="btn btn-secondary w-full mt-2 text-sm"
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
                  accept=".py,.cpp,.c,.go,.rs,.java"
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
                      : 'border-gray-600 hover:border-primary-500 hover:bg-gray-800/50'
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
                      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
                    </div>
                  )}
                </div>

                {uploadSuccess && (
                  <div className="p-2 bg-green-900/30 border border-green-700 rounded text-sm text-green-300 flex items-center gap-2">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                    </svg>
                    Программа успешно загружена!
                  </div>
                )}

                {uploadError && (
                  <div className="p-2 bg-red-900/30 border border-red-700 rounded text-sm text-red-300 flex items-center gap-2">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
                    </svg>
                    {uploadError}
                  </div>
                )}

                <p className="text-xs text-gray-400 text-center">
                  Поддерживаемые форматы: .py, .cpp, .c, .go, .rs, .java
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
                              <ProgramStatusBadge program={program} />
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

// Match groups component - groups matches by program pair and shows iterations as tabs
function MatchGroups({ matches }: { matches: Match[] }) {
  // Group matches by program pair (program1_id + program2_id)
  const groupedMatches: Record<string, Match[]> = {};

  matches.forEach((match) => {
    // Create a consistent key regardless of which program is first
    const ids = [match.program1_id, match.program2_id].sort();
    const key = ids.join('-');

    if (!groupedMatches[key]) {
      groupedMatches[key] = [];
    }
    groupedMatches[key].push(match);
  });

  // Sort matches within each group by created_at
  Object.values(groupedMatches).forEach((group) => {
    group.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
  });

  const groupEntries = Object.entries(groupedMatches);

  if (groupEntries.length === 0) {
    return <p className="text-gray-400">Матчи ещё не проводились.</p>;
  }

  return (
    <div className="space-y-4">
      {groupEntries.map(([key, groupMatches]) => (
        <MatchGroupCard key={key} matches={groupMatches} />
      ))}
    </div>
  );
}

// Match group card with iteration tabs
function MatchGroupCard({ matches }: { matches: Match[] }) {
  const [activeIteration, setActiveIteration] = useState(0);
  const activeMatch = matches[activeIteration];

  // Calculate aggregate stats
  const stats = {
    completed: matches.filter((m) => m.status === 'completed').length,
    pending: matches.filter((m) => m.status === 'pending').length,
    running: matches.filter((m) => m.status === 'running').length,
    failed: matches.filter((m) => m.status === 'failed').length,
    total1: matches.reduce((sum, m) => sum + (m.score1 ?? 0), 0),
    total2: matches.reduce((sum, m) => sum + (m.score2 ?? 0), 0),
    wins1: matches.filter((m) => m.winner === 1).length,
    wins2: matches.filter((m) => m.winner === 2).length,
    draws: matches.filter((m) => m.winner === 0).length,
  };

  const getIterationStatus = (match: Match) => {
    switch (match.status) {
      case 'completed':
        if (match.winner === 1) return 'bg-green-500';
        if (match.winner === 2) return 'bg-red-500';
        return 'bg-gray-400';
      case 'running':
        return 'bg-blue-500 animate-pulse';
      case 'failed':
        return 'bg-red-600';
      default:
        return 'bg-gray-600';
    }
  };

  return (
    <div className="bg-gray-800/50 rounded-lg p-4 border border-gray-700">
      {/* Header with aggregate stats */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-4 gap-2">
        <div className="flex items-center gap-4">
          <div className="text-center">
            <p className="text-sm font-medium text-gray-400 truncate max-w-[120px]" title={matches[0]?.program1_id}>
              Программа 1
            </p>
            <p className="text-2xl font-bold text-gray-100">{stats.total1}</p>
          </div>
          <div className="text-center text-gray-500">
            <span className="text-lg">vs</span>
          </div>
          <div className="text-center">
            <p className="text-sm font-medium text-gray-400 truncate max-w-[120px]" title={matches[0]?.program2_id}>
              Программа 2
            </p>
            <p className="text-2xl font-bold text-gray-100">{stats.total2}</p>
          </div>
        </div>

        <div className="flex items-center gap-3 text-sm">
          <span className="text-green-400" title="Победы 1">
            W1: {stats.wins1}
          </span>
          <span className="text-gray-400" title="Ничьи">
            D: {stats.draws}
          </span>
          <span className="text-red-400" title="Победы 2">
            W2: {stats.wins2}
          </span>
          <span className="text-gray-500">
            ({stats.completed}/{matches.length})
          </span>
        </div>
      </div>

      {/* Iteration tabs */}
      <div className="mb-3">
        <div className="flex flex-wrap gap-1.5">
          {matches.map((match, index) => (
            <button
              key={match.id}
              onClick={() => setActiveIteration(index)}
              className={`w-8 h-8 rounded-lg text-xs font-medium transition-all ${
                activeIteration === index
                  ? 'ring-2 ring-primary-500 ring-offset-1 ring-offset-gray-800'
                  : 'hover:scale-105'
              }`}
              title={`Итерация ${index + 1}: ${match.status}${match.winner !== undefined ? ` (Победа ${match.winner || 'Ничья'})` : ''}`}
            >
              <div className={`w-full h-full rounded-lg flex items-center justify-center text-white ${getIterationStatus(match)}`}>
                {index + 1}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Active iteration details */}
      {activeMatch && (
        <div className="bg-gray-800 rounded-lg p-3 border border-gray-600">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-300">
              Итерация {activeIteration + 1}
            </span>
            <MatchStatusBadge status={activeMatch.status} />
          </div>

          <div className="flex items-center justify-center gap-4 py-2">
            <span className={`text-xl font-bold ${activeMatch.winner === 1 ? 'text-green-400' : 'text-gray-300'}`}>
              {activeMatch.score1 ?? '-'}
            </span>
            <span className="text-gray-400">:</span>
            <span className={`text-xl font-bold ${activeMatch.winner === 2 ? 'text-green-400' : 'text-gray-300'}`}>
              {activeMatch.score2 ?? '-'}
            </span>
          </div>

          {activeMatch.winner !== undefined && activeMatch.status === 'completed' && (
            <p className="text-center text-sm text-gray-400">
              {activeMatch.winner === 0 ? 'Ничья' : `Победа Программы ${activeMatch.winner}`}
            </p>
          )}

          {activeMatch.error_message && (
            <div className="mt-2 p-2 bg-red-900/30 border border-red-700 rounded text-xs text-red-300">
              {activeMatch.error_message}
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

// Program status badge component
function ProgramStatusBadge({ program }: { program: Program }) {
  switch (program.status) {
    case 'compiling':
      return <span className="text-xs bg-yellow-900/50 text-yellow-300 px-2 py-0.5 rounded">Компилируется…</span>;
    case 'ready':
      return <span className="text-xs bg-green-900/50 text-green-300 px-2 py-0.5 rounded">Готова</span>;
    case 'failed':
      return (
        <span
          className="text-xs bg-red-900/50 text-red-300 px-2 py-0.5 rounded"
          title={program.error_message || 'Ошибка компиляции'}
        >
          Ошибка компиляции
        </span>
      );
    default:
      return null;
  }
}

// Match status badge component
function MatchStatusBadge({ status }: { status: string }) {
  switch (status) {
    case 'pending':
      return <span className="text-xs bg-yellow-900/50 text-yellow-300 px-2 py-0.5 rounded">Ожидание</span>;
    case 'running':
      return <span className="text-xs bg-blue-900/50 text-blue-300 px-2 py-0.5 rounded">Выполняется</span>;
    case 'completed':
      return <span className="text-xs bg-green-900/50 text-green-300 px-2 py-0.5 rounded">Завершён</span>;
    case 'failed':
      return <span className="text-xs bg-red-900/50 text-red-300 px-2 py-0.5 rounded">Ошибка</span>;
    default:
      return null;
  }
}

