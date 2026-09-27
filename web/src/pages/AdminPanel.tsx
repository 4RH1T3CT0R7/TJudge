import { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import { queryKeys } from '../api/queryKeys';
import {
  useGames,
  useTournaments,
  useTournamentGames,
  useTournamentGamesStatus,
  useQueueStats,
  useMatchStatistics,
  useSystemMetrics,
  useFullSystemStatus,
} from '../hooks/queries';
import { useAuthStore } from '../store/authStore';
import { SpaceInvader } from '../components/SpaceInvader';
import type { InvaderPose } from '../components/SpaceInvader';
import { useSequenceTyping } from '../hooks/useEasterEggs';
import { Spinner } from '../components/ui/Spinner';
import { ErrorState } from '../components/ui/ErrorState';
import { PageHeader } from '../components/ui/PageHeader';
import { useDelayedLoading } from '../hooks/useDelayedLoading';
import { GamesTab } from '../components/admin/GamesTab';
import { TournamentsTab } from '../components/admin/TournamentsTab';
import { ProgramsTab } from '../components/admin/ProgramsTab';
import { SystemTab } from '../components/admin/SystemTab';
import { Tabs } from '../components/ui/Tabs';
import type { TabItem } from '../components/ui/Tabs';
import { useTabParam } from '../hooks/useTabParam';
import { buildProgramRows } from '../components/admin/programRows';
import type { ProgramRow } from '../components/admin/programRows';
import type { Game } from '../types';

const TAB_IDS = ['games', 'tournaments', 'programs', 'system'] as const;
type AdminTab = (typeof TAB_IDS)[number];

/** Интервал поллинга вкладки «Система». TanStack приостанавливает его в фоновой вкладке браузера. */
const SYSTEM_POLL_INTERVAL = 10_000;

const SUDO_PHRASES = [
  '// I\'m in.',
  '// access granted',
  '// hack the planet',
  '// follow the white rabbit',
  '// the matrix has you',
  '// wake up, Neo',
  '// there is no spoon',
  '// sudo make me a sandwich',
  '// rm -rf doubts',
  '// ping reality',
  '// root@tjudge:~#',
  '// 01101000 01100001',
  '// all your base',
  '// we\'re in the mainframe',
];

export function AdminPanel() {
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const [activeTab, setActiveTab] = useTabParam(TAB_IDS, 'games');
  const queryClient = useQueryClient();

  const gamesQuery = useGames();
  const tournamentsQuery = useTournaments();
  const games = gamesQuery.data ?? [];
  const tournaments = tournamentsQuery.data ?? [];
  const isLoading = gamesQuery.isLoading || tournamentsQuery.isLoading;
  const showLoading = useDelayedLoading(isLoading);

  // Game form state
  const [showGameForm, setShowGameForm] = useState(false);
  const [editingGame, setEditingGame] = useState<Game | null>(null);
  const [gameForm, setGameForm] = useState({
    name: '',
    display_name: '',
    rules: '',
  });
  const [isSavingGame, setIsSavingGame] = useState(false);
  const [gameError, setGameError] = useState<string | null>(null);

  // Tournament form state
  const [showTournamentForm, setShowTournamentForm] = useState(false);
  const [tournamentForm, setTournamentForm] = useState({
    name: '',
    description: '',
    game_type: '',
    max_team_size: 3,
    max_participants: '',
    is_permanent: false,
    start_time: '',
    end_time: '',
  });
  const [selectedGameIds, setSelectedGameIds] = useState<string[]>([]);
  const [isSavingTournament, setIsSavingTournament] = useState(false);
  const [tournamentError, setTournamentError] = useState<string | null>(null);

  // Delete confirmation
  const [deleteGameId, setDeleteGameId] = useState<string | null>(null);
  const [deleteTournamentId, setDeleteTournamentId] = useState<string | null>(null);

  // Admin invader state
  const [adminPose] = useState<InvaderPose>('idle');
  const [adminSpeech, setAdminSpeech] = useState<string | null>('// приветствую, admin');
  const [speechVisible, setSpeechVisible] = useState(true);
  const adminTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const idleTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Helper: set invader reaction (speech only, no pose change to avoid layout shift)
  const setAdminReaction = useCallback((_pose: InvaderPose, speech: string | null, duration = 3000) => {
    clearTimeout(adminTimerRef.current);
    clearTimeout(idleTimerRef.current);
    setAdminSpeech(speech);
    setSpeechVisible(true);
    idleTimerRef.current = setTimeout(() => {
      setSpeechVisible(false);
      setTimeout(() => {
        setAdminSpeech(null);
      }, 300);
    }, duration);
  }, []);

  // Hide initial speech after 2.5s
  useEffect(() => {
    adminTimerRef.current = setTimeout(() => {
      setSpeechVisible(false);
      setTimeout(() => setAdminSpeech(null), 300);
    }, 2500);
    return () => {
      clearTimeout(adminTimerRef.current);
      clearTimeout(idleTimerRef.current);
    };
  }, []);

  // "sudo" easter egg - полностью зелёная hacker-тема
  const [sudoMode, setSudoMode] = useState(false);
  const [sudoActivating, setSudoActivating] = useState(false);
  const sudoCanvasRef = useRef<HTMLCanvasElement>(null);

  // Смена вкладки с реакцией захватчика (в sudo-режиме пропускается - хакерские фразы перехватывают управление)
  const handleTabChange = (tab: AdminTab) => {
    setActiveTab(tab);
    if (sudoMode) return;
    const reactions: Record<AdminTab, [InvaderPose, string]> = {
      games: ['idle', '// game manager'],
      tournaments: ['idle', '// tournaments loaded'],
      programs: ['typing', '// сканирую код...'],
      system: ['idle', '// мониторинг...'],
    };
    const [pose, speech] = reactions[tab];
    setAdminReaction(pose, speech, 2500);
  };

  useSequenceTyping('sudo', useCallback(() => {
    if (sudoMode || sudoActivating) return;
    setSudoActivating(true);
    setAdminReaction('transform', '// ROOT ACCESS GRANTED', 4000);

    // Phase 1: cinematic activation
    setTimeout(() => {
      setSudoMode(true);
      setSudoActivating(false);
    }, 2000);
  }, [sudoMode, sudoActivating, setAdminReaction]));

  // Sudo matrix rain canvas
  useEffect(() => {
    if (!sudoMode) return;
    const canvas = sudoCanvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const CHARS = 'アイウエオカキクケコサシスセソタチツテト0123456789ABCDEF';
    let columns: number[] = [];
    let raf: number;

    const resize = () => {
      canvas.width = canvas.parentElement?.clientWidth || window.innerWidth;
      canvas.height = canvas.parentElement?.clientHeight || window.innerHeight;
      const cols = Math.floor(canvas.width / 14);
      columns = Array(cols).fill(0).map(() => Math.random() * canvas.height);
    };

    resize();
    window.addEventListener('resize', resize);

    const draw = () => {
      ctx.fillStyle = 'rgba(0, 0, 0, 0.06)';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.font = '12px monospace';

      columns.forEach((y, i) => {
        const char = CHARS[Math.floor(Math.random() * CHARS.length)];
        ctx.fillStyle = Math.random() > 0.85 ? '#00ff41' : 'rgba(0,255,65,0.3)';
        ctx.fillText(char, i * 14, y);
        if (y > canvas.height && Math.random() > 0.975) {
          columns[i] = 0;
        } else {
          columns[i] = y + 14;
        }
      });

      raf = requestAnimationFrame(draw);
    };

    raf = requestAnimationFrame(draw);
    return () => {
      cancelAnimationFrame(raf);
      window.removeEventListener('resize', resize);
    };
  }, [sudoMode]);

  useEffect(() => {
    if (!sudoMode) return;
    const tick = () => {
      const phrase = SUDO_PHRASES[Math.floor(Math.random() * SUDO_PHRASES.length)];
      setAdminReaction('idle', phrase, 4000);
    };
    const first = setTimeout(tick, 5000);
    const interval = setInterval(tick, 8000 + Math.random() * 4000);
    return () => { clearTimeout(first); clearInterval(interval); };
  }, [sudoMode, setAdminReaction]);

  // Action errors
  const [actionError, setActionError] = useState<string | null>(null);

  // Tournament games management state: данные модалки тянут query-хуки, пока managingTournamentId задан
  const [managingTournamentId, setManagingTournamentId] = useState<string | null>(null);
  const managingGamesQuery = useTournamentGames(managingTournamentId ?? '');
  const managingStatusQuery = useTournamentGamesStatus(managingTournamentId ?? '');
  const managingTournamentGames = managingGamesQuery.data ?? [];
  const managingTournamentGamesStatus = managingStatusQuery.data ?? [];
  const isLoadingTournamentGames = managingGamesQuery.isLoading || managingStatusQuery.isLoading;
  const showLoadingTournamentGames = useDelayedLoading(isLoadingTournamentGames);
  const [runningGameMatches, setRunningGameMatches] = useState<string | null>(null);
  const [settingActiveGame, setSettingActiveGame] = useState<string | null>(null);
  const [resettingGame, setResettingGame] = useState<string | null>(null);

  // Programs tab state: композитный запрос (игры турнира, команды, лидерборды и программы игр).
  // Ключ лежит в поддереве queryKeys.tournament(id), поэтому invalidate по турниру сбрасывает и его.
  // Запросы по играм независимы: сбой одного не прячет остальные.
  const [selectedTournamentId, setSelectedTournamentId] = useState<string | null>(null);
  const programsQuery = useQuery({
    queryKey: [...queryKeys.tournament(selectedTournamentId ?? ''), 'admin-programs'],
    enabled: !!selectedTournamentId,
    queryFn: async () => {
      const tournamentId = selectedTournamentId;
      if (!tournamentId) throw new Error('tournament not selected');

      const [games, teams] = await Promise.all([
        api.getTournamentGames(tournamentId),
        api.getTournamentTeams(tournamentId).catch(() => []),
      ]);
      const rowsByGame: Record<string, ProgramRow[]> = {};
      await Promise.all(games.map(async (game) => {
        const [leaderboard, programs] = await Promise.all([
          api.getGameLeaderboard(tournamentId, game.id).catch(() => []),
          api.getGamePrograms(tournamentId, game.id).catch(() => []),
        ]);
        rowsByGame[game.id] = buildProgramRows(programs ?? [], leaderboard ?? [], teams ?? []);
      }));

      return { games, rowsByGame };
    },
  });
  const tournamentGames = programsQuery.data?.games ?? [];
  const programRows = programsQuery.data?.rowsByGame ?? {};
  const isLoadingPrograms = programsQuery.isLoading;
  const showLoadingPrograms = useDelayedLoading(isLoadingPrograms);

  // System tab state: поллинг через refetchInterval самих запросов (только на активной вкладке
  // «Система»; в фоновой вкладке браузера TanStack приостанавливает интервал сам).
  // Each request is independent to avoid cascading failures.
  const isSystemTab = activeTab === 'system';
  const queueStatsQuery = useQueueStats({ enabled: isSystemTab, pollInterval: SYSTEM_POLL_INTERVAL });
  const matchStatsQuery = useMatchStatistics(undefined, { enabled: isSystemTab, pollInterval: SYSTEM_POLL_INTERVAL });
  const systemMetricsQuery = useSystemMetrics({ enabled: isSystemTab, pollInterval: SYSTEM_POLL_INTERVAL });
  const fullStatusQuery = useFullSystemStatus({ enabled: isSystemTab, pollInterval: SYSTEM_POLL_INTERVAL });
  const failedMatchesQuery = useQuery({
    queryKey: queryKeys.failedMatches,
    queryFn: () => api.getFailedMatches(50),
    enabled: isSystemTab,
    refetchInterval: SYSTEM_POLL_INTERVAL,
  });

  const queueStats = queueStatsQuery.data ?? null;
  const matchStats = matchStatsQuery.data ?? null;
  const systemMetrics = systemMetricsQuery.data ?? null;
  const failedMatches = failedMatchesQuery.data ?? [];
  const fullStatus = fullStatusQuery.data ?? null;
  const isLoadingSystem =
    queueStatsQuery.isFetching ||
    matchStatsQuery.isFetching ||
    systemMetricsQuery.isFetching ||
    failedMatchesQuery.isFetching ||
    fullStatusQuery.isFetching;
  const showLoadingSystem = useDelayedLoading(isLoadingSystem);
  // Show fetch error only if all requests failed
  const allSystemQueriesFailed =
    queueStatsQuery.isError &&
    matchStatsQuery.isError &&
    systemMetricsQuery.isError &&
    failedMatchesQuery.isError &&
    fullStatusQuery.isError;

  const [systemError, setSystemError] = useState<string | null>(null);
  const [isClearing, setIsClearing] = useState(false);
  const [isPurging, setIsPurging] = useState(false);
  // Какая кнопка восстановления сейчас выполняется (ключ действия или null).
  const [recoveryBusy, setRecoveryBusy] = useState<string | null>(null);

  useEffect(() => {
    // Redirect non-admin users
    if (user && user.role !== 'admin') {
      navigate('/');
    }
  }, [user, navigate]);

  // Принудительное обновление данных вкладки «Система» (кнопка «Обновить» и пост-мутации).
  const refreshSystemData = useCallback(() => {
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.queueStats }),
      queryClient.invalidateQueries({ queryKey: queryKeys.matchStatistics() }),
      queryClient.invalidateQueries({ queryKey: queryKeys.systemMetrics }),
      queryClient.invalidateQueries({ queryKey: queryKeys.failedMatches }),
      queryClient.invalidateQueries({ queryKey: queryKeys.fullSystemStatus }),
    ]);
  }, [queryClient]);

  // Invader reactions to system state. Зависимости — сырые data из запросов:
  // structural sharing TanStack сохраняет идентичность при неизменных данных,
  // поэтому реакция срабатывает только на реальные изменения.
  const queueStatsData = queueStatsQuery.data;
  const failedMatchesData = failedMatchesQuery.data;
  /* eslint-disable react-hooks/set-state-in-effect -- setAdminReaction (реплика захватчика) намеренно реагирует на смену данных запросов */
  useEffect(() => {
    if (activeTab !== 'system') return;
    if (failedMatchesData && failedMatchesData.length > 0) {
      setAdminReaction('dizzy', '// ошибки!', 4000);
    } else if (queueStatsData && queueStatsData.total > 50) {
      setAdminReaction('run', '// очередь растёт!', 4000);
    } else if (queueStatsData && queueStatsData.total === 0) {
      setAdminReaction('idle', '// всё чисто', 3000);
    }
  }, [activeTab, failedMatchesData, queueStatsData, setAdminReaction]);
  /* eslint-enable react-hooks/set-state-in-effect */

  if (user?.role !== 'admin') {
    return (
      <ErrorState message="Доступ запрещён: нужны права администратора" />
    );
  }

  if (showLoading) {
    return <div className="flex justify-center py-24 text-sm text-gray-400"><Spinner>загрузка админки</Spinner></div>;
  }

  if (isLoading) {
    return null;
  }

  const tabs: TabItem<AdminTab>[] = [
    { id: 'games', label: 'Игры', count: games.length },
    { id: 'tournaments', label: 'Турниры', count: tournaments.length },
    { id: 'programs', label: 'Программы' },
    { id: 'system', label: 'Система' },
  ];

  return (
    <div className={`relative ${sudoMode ? 'sudo-theme' : ''} ${sudoActivating ? 'sudo-activating' : ''}`}>
      {/* Sudo matrix rain background */}
      {sudoMode && (
        <canvas
          ref={sudoCanvasRef}
          className="absolute inset-0 pointer-events-none opacity-[0.07] z-0"
          style={{ width: '100%', height: '100%' }}
        />
      )}
      {/* Sudo scanline overlay */}
      {sudoMode && (
        <div className="sudo-scanlines absolute inset-0 pointer-events-none z-[1]" />
      )}
      {/* Sudo CRT vignette */}
      {sudoMode && (
        <div className="sudo-vignette absolute inset-0 pointer-events-none z-[1]" />
      )}

      <div className="relative z-[2]">
      <PageHeader
        crumbs={[{ label: 'админка' }]}
        title={
          <>
            <title>Админка — TJudge</title>
            {sudoMode ? <span className="sudo-text">root@tjudge:~# admin</span> : 'Панель администратора'}
          </>
        }
        actions={
          /* на узком экране реплика маскота вылезала бы за край */
          <div className="relative hidden sm:block">
            <SpaceInvader size="sm" controlledPose={adminPose} speechBubble={speechVisible ? adminSpeech : null} colorOverride={sudoMode ? '#00ff41' : null} />
          </div>
        }
      />

      <Tabs label="Разделы админки" items={tabs} active={activeTab} onChange={handleTabChange}>
        {/* Games Tab */}
        {activeTab === 'games' && (
          <GamesTab
            games={games}
            showGameForm={showGameForm}
            setShowGameForm={setShowGameForm}
            editingGame={editingGame}
            setEditingGame={setEditingGame}
            gameForm={gameForm}
            setGameForm={setGameForm}
            isSavingGame={isSavingGame}
            setIsSavingGame={setIsSavingGame}
            gameError={gameError}
            setGameError={setGameError}
            deleteGameId={deleteGameId}
            setDeleteGameId={setDeleteGameId}
            setAdminReaction={setAdminReaction}
          />
        )}

        {/* Tournaments Tab */}
        {activeTab === 'tournaments' && (
          <TournamentsTab
            tournaments={tournaments}
            games={games}
            showTournamentForm={showTournamentForm}
            setShowTournamentForm={setShowTournamentForm}
            tournamentForm={tournamentForm}
            setTournamentForm={setTournamentForm}
            selectedGameIds={selectedGameIds}
            setSelectedGameIds={setSelectedGameIds}
            isSavingTournament={isSavingTournament}
            setIsSavingTournament={setIsSavingTournament}
            tournamentError={tournamentError}
            setTournamentError={setTournamentError}
            deleteTournamentId={deleteTournamentId}
            setDeleteTournamentId={setDeleteTournamentId}
            actionError={actionError}
            setActionError={setActionError}
            managingTournamentId={managingTournamentId}
            setManagingTournamentId={setManagingTournamentId}
            managingTournamentGames={managingTournamentGames}
            managingTournamentGamesStatus={managingTournamentGamesStatus}
            isLoadingTournamentGames={isLoadingTournamentGames}
            showLoadingTournamentGames={showLoadingTournamentGames}
            runningGameMatches={runningGameMatches}
            setRunningGameMatches={setRunningGameMatches}
            settingActiveGame={settingActiveGame}
            setSettingActiveGame={setSettingActiveGame}
            resettingGame={resettingGame}
            setResettingGame={setResettingGame}
            setAdminReaction={setAdminReaction}
          />
        )}

        {/* Programs Tab */}
        {activeTab === 'programs' && (
          <ProgramsTab
            tournaments={tournaments}
            selectedTournamentId={selectedTournamentId}
            setSelectedTournamentId={setSelectedTournamentId}
            tournamentGames={tournamentGames}
            programRows={programRows}
            isLoadingPrograms={isLoadingPrograms}
            showLoadingPrograms={showLoadingPrograms}
            setActionError={setActionError}
            setAdminReaction={setAdminReaction}
          />
        )}

        {/* System Tab */}
        {activeTab === 'system' && (
          <SystemTab
            queueStats={queueStats}
            matchStats={matchStats}
            systemMetrics={systemMetrics}
            failedMatches={failedMatches}
            fullStatus={fullStatus}
            fullStatusIsError={fullStatusQuery.isError}
            isLoadingSystem={isLoadingSystem}
            showLoadingSystem={showLoadingSystem}
            allSystemQueriesFailed={allSystemQueriesFailed}
            systemError={systemError}
            setSystemError={setSystemError}
            isClearing={isClearing}
            setIsClearing={setIsClearing}
            isPurging={isPurging}
            setIsPurging={setIsPurging}
            recoveryBusy={recoveryBusy}
            setRecoveryBusy={setRecoveryBusy}
            refreshSystemData={refreshSystemData}
            setAdminReaction={setAdminReaction}
          />
        )}
      </Tabs>
    </div>
    </div>
  );
}
