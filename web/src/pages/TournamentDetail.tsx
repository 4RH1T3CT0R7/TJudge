import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import api from '../api/client';
import { queryKeys } from '../api/queryKeys';
import {
  useTournament,
  useCrossGameLeaderboard,
  useMatchesByRounds,
  useTournamentGames,
  useTournamentGamesStatus,
  useTournamentTeams,
  useMyTeam,
} from '../hooks/queries';
import { useTournamentLive } from '../hooks/useTournamentLive';
import { useToastStore } from '../store/toastStore';
import { confirmDialog } from '../store/confirmStore';
import {
  InfoCircleIcon, ChartBarIcon, PuzzlePieceIcon, UsersIcon, PlayIcon,
  CheckCircleIcon, XMarkIcon, UserPlusIcon,
  ArrowLeftIcon, HashtagIcon, FolderIcon,
} from '../components/icons';
import { useAuthStore } from '../store/authStore';
import { SpaceInvader } from '../components/SpaceInvader';
import type { InvaderPose } from '../components/SpaceInvader';
import { CinematicOverlay } from '../components/CinematicOverlay';
import { TerminalLoader } from '../components/TerminalLoader';
import { useDelayedLoading } from '../hooks/useDelayedLoading';
import { useOnRoundFinished } from '../hooks/useOnRoundFinished';
import { InfoTab } from '../components/tournament/InfoTab';
import {
  LeaderboardTab,
  GeneralLeaderboardTable,
  CrossGameLeaderboardTableDark,
} from '../components/tournament/LeaderboardTab';
import { GamesTab } from '../components/tournament/GamesTab';
import { TeamsTab } from '../components/tournament/TeamsTab';
import { MatchesTab } from '../components/tournament/MatchesTab';
import { JoinTournamentModal } from '../components/tournament/JoinTournamentModal';
import { extractErrorMessage, statusConfig } from '../components/tournament/helpers';
import { useGameAdminActions } from '../components/tournament/useGameAdminActions';
import { handleTabListKeyDown } from '../components/ui/tabs';
import type {
  Tournament,
  TournamentStatus,
  Team,
  Game,
  CrossGameLeaderboardEntry,
  MatchRound,
  TournamentGameWithDetails,
} from '../types';

type TabType = 'info' | 'leaderboard' | 'matches' | 'games' | 'teams';

export function TournamentDetail() {
  const { id } = useParams<{ id: string }>();
  const tournamentId = id ?? '';
  const { isAuthenticated, user } = useAuthStore();
  const queryClient = useQueryClient();

  // Живые обновления: WS-события точечно инвалидируют кэш (useTournamentLive),
  // а pollInterval включается только как fallback, когда WS недоступен.
  const tournamentQuery = useTournament(tournamentId);
  const live = useTournamentLive({
    tournamentId,
    enabled: isAuthenticated,
  });
  const { isConnected } = live;

  const teamsQuery = useTournamentTeams(tournamentId);
  const gamesQuery = useTournamentGames(tournamentId);
  const leaderboardQuery = useCrossGameLeaderboard(tournamentId, { pollInterval: live.pollInterval });
  const matchRoundsQuery = useMatchesByRounds(tournamentId, { pollInterval: live.pollInterval });
  const gamesStatusQuery = useTournamentGamesStatus(tournamentId, { pollInterval: live.pollInterval });
  const myTeamQuery = useMyTeam(tournamentId, { enabled: isAuthenticated });

  const tournament: Tournament | null = tournamentQuery.data ?? null;
  const teams: Team[] = teamsQuery.data ?? [];
  const games: Game[] = gamesQuery.data ?? [];
  const crossGameLeaderboard: CrossGameLeaderboardEntry[] = leaderboardQuery.data ?? [];
  const matchRounds: MatchRound[] = matchRoundsQuery.data ?? [];
  const gamesStatus: TournamentGameWithDetails[] = gamesStatusQuery.data ?? [];
  const myTeam: Team | null = myTeamQuery.data ?? null;

  const [activeTab, setActiveTab] = useState<TabType>('info');
  // Первичная загрузка всех данных страницы (раньше - единый ручной флаг).
  // isLoading у disabled-запросов (myTeam без авторизации) - false.
  const isLoading =
    tournamentQuery.isLoading ||
    teamsQuery.isLoading ||
    gamesQuery.isLoading ||
    leaderboardQuery.isLoading ||
    matchRoundsQuery.isLoading ||
    gamesStatusQuery.isLoading ||
    myTeamQuery.isLoading;
  const showLoading = useDelayedLoading(isLoading);
  const error = tournamentQuery.isError ? 'Не удалось загрузить данные турнира' : null;
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showCrossGameLeaderboard, setShowCrossGameLeaderboard] = useState(true); // По играм / Общий
  const [isRetryingMatches, setIsRetryingMatches] = useState(false);

  // Join modal state
  const [showJoinModal, setShowJoinModal] = useState(false);
  const [teamName, setTeamName] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [isJoining, setIsJoining] = useState(false);
  const [joinError, setJoinError] = useState('');

  // Action states
  const [isStarting, setIsStarting] = useState(false);
  const [isCompleting, setIsCompleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  // Games status state (for active game management): состояние и обработчики в хуке.
  const {
    runningGameId,
    settingActiveGameId,
    resettingGameId,
    handleRunGameMatches,
    handleSetActiveGame,
    handleResetGameRound,
  } = useGameAdminActions({ tournamentId, tournament, games, setActionError });

  // Invader state for WS reactions
  const [wsInvaderPose, setWsInvaderPose] = useState<InvaderPose>('idle');
  const wsInvaderTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Cinematic state
  const [cinematicType, setCinematicType] = useState<'tournament_victory' | null>(null);
  const [cinematicTeamName, setCinematicTeamName] = useState('');
  const [top1Banner, setTop1Banner] = useState<string | null>(null);
  // маскот во вкладках реагирует на живые обновления не чаще раза в 15 с
  const lastLiveFlashRef = useRef(0);

  const flashWsInvader = useCallback((pose: InvaderPose, duration = 2000) => {
    clearTimeout(wsInvaderTimerRef.current);
    setWsInvaderPose(pose);
    wsInvaderTimerRef.current = setTimeout(() => setWsInvaderPose('idle'), duration);
  }, []);

  // Реакции инвейдера и синематики: раньше триггерились WS-сообщениями напрямую,
  // теперь - изменением данных в кэше (WS-инвалидации useTournamentLive или fallback-поллинг
  // приводят к рефетчу; structural sharing TanStack Query меняет ссылку только при
  // реальном изменении данных, поэтому ложных срабатываний нет).
  const leaderboardData = leaderboardQuery.data;
  const prevLeaderboardDataRef = useRef<CrossGameLeaderboardEntry[] | undefined>(undefined);
  useEffect(() => {
    if (!leaderboardData) return;
    const prev = prevLeaderboardDataRef.current;
    prevLeaderboardDataRef.current = leaderboardData;
    if (prev && prev !== leaderboardData && Date.now() - lastLiveFlashRef.current > 15_000) {
      lastLiveFlashRef.current = Date.now();
      flashWsInvader('attack', 800);
    }
  }, [leaderboardData, flashWsInvader]);

  // «#1» - только по итогам раунда: посреди раунда суммы частичные. Лидерборд
  // перечитывается уже после того, как раунды показали конец: запрошенный вместе
  // с ними мог не застать последние матчи
  const roundRunning = (matchRoundsQuery.data ?? []).some(r => r.pending_count > 0 || r.running_count > 0);
  const refetchLeaderboardData = leaderboardQuery.refetch;
  useOnRoundFinished(roundRunning, () => {
    if (!myTeam) return;
    void refetchLeaderboardData().then(({ data }) => {
      if (data?.find(e => e.team_id === myTeam.id)?.rank === 1) {
        setTop1Banner(`> ${myTeam.name} — #1 по итогам раунда`);
      }
    });
  });
  useEffect(() => {
    if (!top1Banner) return;
    const t = setTimeout(() => setTop1Banner(null), 2500);
    return () => clearTimeout(t);
  }, [top1Banner]);

  const tournamentStatusValue = tournament?.status;
  const prevTournamentStatusRef = useRef<TournamentStatus | undefined>(undefined);
  useEffect(() => {
    if (!tournamentStatusValue) return;
    const prev = prevTournamentStatusRef.current;
    prevTournamentStatusRef.current = tournamentStatusValue;
    if (prev && prev !== tournamentStatusValue) {
      flashWsInvader('handsUp', 2000);

      // Check for tournament completion with user's team at #1
      if (tournamentStatusValue === 'completed' && myTeam && leaderboardData && leaderboardData.length > 0) {
        const userEntry = leaderboardData.find(e => e.team_id === myTeam.id);
        if (userEntry && userEntry.rank === 1) {
          setCinematicTeamName(myTeam.name);
          setCinematicType('tournament_victory');
        }
      }
    }
  }, [tournamentStatusValue, myTeam, leaderboardData, flashWsInvader]);

  // Инвалидация всего поддерева турнира: детали, лидерборды, матчи, games-status, команды, my-team.
  const invalidateTournamentData = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.tournament(tournamentId) });
  }, [queryClient, tournamentId]);

  const handleCreateTeam = async () => {
    if (!tournamentId || !teamName.trim()) return;

    setIsJoining(true);
    try {
      const team = await api.createTeam(tournamentId, teamName.trim());
      queryClient.setQueryData<Team | null>(queryKeys.myTeam(tournamentId), team);
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentTeams(tournamentId) });
      setShowJoinModal(false);
      setTeamName('');
    } catch (err) {
      console.error('Failed to create team:', err);
    } finally {
      setIsJoining(false);
    }
  };

  const handleJoinTeam = async () => {
    if (!joinCode.trim()) return;

    setIsJoining(true);
    setJoinError('');
    try {
      const team = await api.joinTeamByCode(joinCode.trim());
      queryClient.setQueryData<Team | null>(queryKeys.myTeam(tournamentId), team);
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentTeams(tournamentId) });
      setShowJoinModal(false);
      setJoinCode('');
    } catch {
      setJoinError('Неверный код');
    } finally {
      setIsJoining(false);
    }
  };

  const toggleFullscreen = () => {
    setIsFullscreen(!isFullscreen);
  };

  const handleStartTournament = async () => {
    if (!tournament) return;

    setIsStarting(true);
    setActionError(null);
    try {
      await api.startTournament(tournament.id);
      invalidateTournamentData();
    } catch (err: unknown) {
      console.error('Failed to start tournament:', err);
      setActionError(extractErrorMessage(err, 'Не удалось запустить турнир'));
    } finally {
      setIsStarting(false);
    }
  };

  const handleCompleteTournament = async () => {
    if (!tournament) return;

    setIsCompleting(true);
    setActionError(null);
    try {
      await api.completeTournament(tournament.id);
      invalidateTournamentData();
    } catch (err: unknown) {
      console.error('Failed to complete tournament:', err);
      setActionError(extractErrorMessage(err, 'Не удалось завершить турнир'));
    } finally {
      setIsCompleting(false);
    }
  };

  const handleRetryFailedMatches = async () => {
    if (!tournament) return;

    setIsRetryingMatches(true);
    setActionError(null);
    try {
      const result = await api.retryFailedMatches(tournament.id);
      setActionError(null);
      useToastStore.getState().addToast(`Перезапущено ${result.enqueued} неудачных матчей`, 'success');
      invalidateTournamentData();
    } catch (err: unknown) {
      console.error('Failed to retry matches:', err);
      setActionError(extractErrorMessage(err, 'Не удалось перезапустить матчи'));
    } finally {
      setIsRetryingMatches(false);
    }
  };

  // Ручное обновление матчей (кнопка «Обновить»): счётчики и открытые страницы раундов.
  const refreshMatches = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
  }, [queryClient, tournamentId]);
  const isRefreshingMatches = matchRoundsQuery.isRefetching;

  // Ручное обновление таблиц рейтинга (кнопка «Обновить»).
  const refetchLeaderboard = leaderboardQuery.refetch;
  const refreshLeaderboard = useCallback(() => {
    void refetchLeaderboard();
  }, [refetchLeaderboard]);
  const isRefreshingLeaderboard = leaderboardQuery.isRefetching;

  if (showLoading) {
    return <TerminalLoader />;
  }

  if (isLoading) {
    return null;
  }

  if (error || !tournament) {
    return (
      <div className="text-center py-24">
        <div className="flex justify-center mb-4">
          <SpaceInvader size="sm" controlledPose="cry" speechBubble="// ошибка" eyeOverride="sad" />
        </div>
        <p className="text-red-500 text-lg mb-4">{error || 'Турнир не найден'}</p>
        <Link to="/tournaments" className="btn btn-secondary">
          <ArrowLeftIcon />
          Назад к турнирам
        </Link>
      </div>
    );
  }

  const totalMatches = matchRounds.reduce((sum, r) => sum + r.total_matches, 0);
  const activeGame = gamesStatus.find(g => g.is_active) || null;
  const activeGameHasRunningMatches = activeGame && matchRounds.some(
    r => r.game_type === activeGame.game_name && (r.pending_count > 0 || r.running_count > 0)
  );
  const tabs: { id: TabType; label: string; icon: React.FC; count?: number }[] = [
    { id: 'info', label: 'Информация', icon: InfoCircleIcon },
    { id: 'leaderboard', label: 'Таблица', icon: ChartBarIcon },
    { id: 'matches', label: 'Матчи', icon: FolderIcon, count: totalMatches },
    { id: 'games', label: 'Игры', icon: PuzzlePieceIcon, count: games.length },
    { id: 'teams', label: 'Команды', icon: UsersIcon, count: teams.length },
  ];

  const isCreator = user?.id === tournament.creator_id;
  const isAdmin = user?.role === 'admin';
  const canManage = isCreator || isAdmin;
  const canStart = canManage && tournament.status === 'pending';
  const canComplete = canManage && tournament.status === 'active';
  const config = statusConfig[tournament.status];

  // Fullscreen leaderboard view
  if (isFullscreen) {
    return (
      <div className="fixed inset-0 bg-gray-900 text-white z-50 overflow-auto">
        <div className="p-4 md:p-6">
          <div className="flex justify-between items-center mb-4">
            <div>
              <title>{`${tournament.name} — TJudge`}</title>
              <h1 className="text-3xl md:text-4xl font-bold mb-2">{tournament.name}</h1>
              <p className="text-gray-400">
                {showCrossGameLeaderboard ? 'Рейтинг по играм' : 'Общий рейтинг'}
              </p>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex gap-2">
                <button
                  onClick={() => setShowCrossGameLeaderboard(true)}
                  aria-pressed={showCrossGameLeaderboard}
                  className={`btn text-sm ${showCrossGameLeaderboard ? 'bg-primary-600 hover:bg-primary-700' : 'bg-gray-700 hover:bg-gray-600'} text-white`}
                >
                  По играм
                </button>
                <button
                  onClick={() => setShowCrossGameLeaderboard(false)}
                  aria-pressed={!showCrossGameLeaderboard}
                  className={`btn text-sm ${!showCrossGameLeaderboard ? 'bg-primary-600 hover:bg-primary-700' : 'bg-gray-700 hover:bg-gray-600'} text-white`}
                >
                  Общий
                </button>
              </div>
              {isConnected && (
                <span className="online-indicator text-green-400">
                  Обновления в реальном времени
                </span>
              )}
              <button onClick={toggleFullscreen} className="btn bg-gray-700 hover:bg-gray-600 text-white">
                <XMarkIcon />
                Закрыть
              </button>
            </div>
          </div>
          {showCrossGameLeaderboard ? (
            <CrossGameLeaderboardTableDark entries={crossGameLeaderboard} games={games} />
          ) : (
            <GeneralLeaderboardTable entries={crossGameLeaderboard} isDark />
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      {/* Cinematic overlay for tournament victory */}
      {cinematicType && (
        <CinematicOverlay
          type={cinematicType}
          teamName={cinematicTeamName}
          onComplete={() => setCinematicType(null)}
        />
      )}

      {/* «#1» по итогам раунда: немодальная строка, таблицу не закрывает */}
      <div role="status" className="fixed top-20 inset-x-0 z-40 flex justify-center pointer-events-none px-4">
        {top1Banner && (
          <p className="px-4 py-2 rounded-lg border border-amber-500/50 bg-gray-900/95 font-mono text-sm text-amber-400 shadow-lg animate-fade-in">
            {top1Banner}
          </p>
        )}
      </div>

      {/* Header */}
      <div className="mb-8">
        <Link to="/tournaments" className="inline-flex items-center gap-2 text-gray-400 hover:text-primary-400 mb-4 transition-colors">
          <ArrowLeftIcon />
          <span>Назад к турнирам</span>
        </Link>

        <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
          <div>
            <div className="flex flex-wrap items-center gap-3 mb-3">
              <title>{`${tournament.name} — TJudge`}</title>
              <h1 className="text-3xl font-bold text-gray-100">{tournament.name}</h1>
              <span className={config.badge}>
                {config.label}
              </span>
              {tournament.is_permanent && (
                <span className="badge badge-blue">
                  Постоянный
                </span>
              )}
            </div>
            <div className="flex items-center gap-2 text-gray-300">
              <HashtagIcon className="w-4 h-4" />
              <span>Код:</span>
              <code className="bg-gray-800 px-3 py-1 rounded-lg font-mono text-gray-100">
                {tournament.code}
              </code>
            </div>
          </div>

          <div className="flex flex-wrap gap-3">
            {isAuthenticated && !myTeam && tournament.status === 'pending' && (
              <button onClick={() => setShowJoinModal(true)} className="btn btn-primary">
                <UserPlusIcon />
                Участвовать
              </button>
            )}
            {canStart && (
              <button
                onClick={handleStartTournament}
                disabled={isStarting}
                className="btn btn-success"
              >
                <PlayIcon />
                {isStarting ? 'Запуск...' : 'Запустить турнир'}
              </button>
            )}
            {canComplete && (
              <button
                onClick={handleCompleteTournament}
                disabled={isCompleting}
                className="btn btn-secondary"
              >
                <CheckCircleIcon />
                {isCompleting ? 'Завершение...' : 'Завершить турнир'}
              </button>
            )}
            {isAdmin && tournament.status === 'active' && (
              <>
                <button
                  onClick={() => {
                    if (activeGame) {
                      handleRunGameMatches(activeGame.game_id, activeGame.game_name, activeGame.game_display_name);
                    }
                  }}
                  disabled={!!runningGameId || !activeGame || !!activeGameHasRunningMatches}
                  className="btn btn-primary"
                >
                  <PlayIcon />
                  {runningGameId
                    ? 'Запуск...'
                    : activeGameHasRunningMatches
                      ? 'Раунд выполняется...'
                      : activeGame
                        ? 'Запустить раунд'
                        : 'Нет активной игры'}
                </button>
                <button
                  onClick={handleRetryFailedMatches}
                  disabled={isRetryingMatches}
                  className="btn btn-warning"
                >
                  {isRetryingMatches ? 'Перезапуск...' : 'Перезапустить неудачные'}
                </button>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Action Error */}
      {actionError && (
        <div className="alert alert-error mb-6 animate-slide-up">
          <XMarkIcon />
          <p>{actionError}</p>
        </div>
      )}

      {/* My Team Badge */}
      {myTeam && (
        <div className="alert alert-info mb-6 animate-slide-up">
          <UsersIcon />
          <div className="flex-1">
            <p>
              Ваша команда: <strong>{myTeam.name}</strong>
            </p>
          </div>
          <Link to={`/teams/${myTeam.id}`} className="btn btn-primary text-sm">
            Управление командой
          </Link>
        </div>
      )}

      {/* Tabs */}
      <div className="relative bg-gray-900 rounded-lg border border-gray-800 mb-6 p-1.5">
        <nav className="flex gap-1 overflow-x-auto items-center" role="tablist" onKeyDown={handleTabListKeyDown}>
          {tabs.map((tab) => {
            const TabIcon = tab.icon;
            return (
              <button
                key={tab.id}
                role="tab"
                aria-selected={activeTab === tab.id}
                tabIndex={activeTab === tab.id ? 0 : -1}
                onClick={() => setActiveTab(tab.id)}
                className={`tab flex items-center gap-2 whitespace-nowrap ${
                  activeTab === tab.id ? 'tab-active' : 'tab-inactive'
                }`}
              >
                <TabIcon />
                {tab.label}
                {tab.count !== undefined && (
                  <span className={`text-xs px-2 py-0.5 rounded-full ${
                    activeTab === tab.id
                      ? 'bg-white/20 text-white'
                      : 'bg-gray-700 text-gray-300'
                  }`}>
                    {tab.count}
                  </span>
                )}
              </button>
            );
          })}
        </nav>
        {/* маскот живых обновлений вне потока: появление не сдвигает вкладки.
            Только с lg, где справа от вкладок есть место, уменьшен до высоты
            панели и без реплики: иначе заходил на вкладки и баннер команды */}
        {isConnected && wsInvaderPose !== 'idle' && (
          <div className="absolute right-3 top-1/2 -translate-y-1/2 scale-50 origin-right pointer-events-none hidden lg:block">
            <SpaceInvader size="sm" controlledPose={wsInvaderPose} />
          </div>
        )}
      </div>

      {/* Tab Content */}
      <div className="animate-fade-in">
        {activeTab === 'info' && (
          <InfoTab tournament={tournament} />
        )}

        {activeTab === 'leaderboard' && (
          <LeaderboardTab
            crossGameEntries={crossGameLeaderboard}
            games={games}
            isConnected={isConnected}
            showCrossGame={showCrossGameLeaderboard}
            onShowCrossGameChange={setShowCrossGameLeaderboard}
            onToggleFullscreen={toggleFullscreen}
            onRefresh={refreshLeaderboard}
            isRefreshing={isRefreshingLeaderboard}
            hasActiveMatches={roundRunning}
            isCompleted={tournament.status === 'completed'}
          />
        )}

        {activeTab === 'matches' && (
          <MatchesTab
            tournamentId={tournament.id}
            rounds={matchRounds}
            onRefresh={refreshMatches}
            isRefreshing={isRefreshingMatches}
            isAdmin={isAdmin}
            pollInterval={live.pollInterval}
          />
        )}

        {activeTab === 'games' && (
          <GamesTab
            games={games}
            gamesStatus={gamesStatus}
            tournamentId={tournament.id}
            myTeam={myTeam}
            isAdmin={isAdmin}
            tournamentStatus={tournament.status}
            onRunGameMatches={handleRunGameMatches}
            onSetActiveGame={handleSetActiveGame}
            onResetGameRound={handleResetGameRound}
            runningGameId={runningGameId}
            settingActiveGameId={settingActiveGameId}
            resettingGameId={resettingGameId}
            matchRounds={matchRounds}
          />
        )}

        {activeTab === 'teams' && (
          <TeamsTab
            teams={teams}
            isAuthenticated={isAuthenticated}
            isAdmin={isAdmin}
            myTeam={myTeam}
            tournamentStatus={tournament.status}
            onJoinByCode={handleJoinTeam}
            joinCode={joinCode}
            setJoinCode={setJoinCode}
            isJoining={isJoining}
            joinError={joinError}
            setJoinError={setJoinError}
            onDisqualify={async (teamId) => {
              if (!(await confirmDialog({
                title: 'Дисквалификация',
                message: 'Дисквалифицировать команду? Все матчи с её участием будут удалены.',
                confirmLabel: 'Дисквалифицировать',
                danger: true,
              }))) return;
              try {
                await api.disqualifyTeam(teamId);
                invalidateTournamentData();
              } catch (err) {
                console.error('Failed to disqualify team:', err);
                useToastStore.getState().addToast('Не удалось дисквалифицировать команду. Попробуйте снова.', 'error');
              }
            }}
            onRestore={async (teamId) => {
              if (!(await confirmDialog({
                title: 'Восстановление команды',
                message: 'Восстановить команду? Она снова сможет участвовать в турнире.',
                confirmLabel: 'Восстановить',
              }))) return;
              try {
                await api.restoreTeam(teamId);
                invalidateTournamentData();
              } catch (err) {
                console.error('Failed to restore team:', err);
                useToastStore.getState().addToast('Не удалось восстановить команду. Попробуйте снова.', 'error');
              }
            }}
          />
        )}
      </div>

      {/* Join Modal */}
      <JoinTournamentModal
        open={showJoinModal}
        onClose={() => setShowJoinModal(false)}
        teamName={teamName}
        setTeamName={setTeamName}
        joinCode={joinCode}
        setJoinCode={setJoinCode}
        joinError={joinError}
        setJoinError={setJoinError}
        isJoining={isJoining}
        onCreateTeam={handleCreateTeam}
        onJoinTeam={handleJoinTeam}
      />
    </div>
  );
}
