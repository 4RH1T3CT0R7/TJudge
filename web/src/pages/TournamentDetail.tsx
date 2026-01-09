import { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import api from '../api/client';
import { useWebSocket } from '../hooks/useWebSocket';
import { useAuthStore } from '../store/authStore';
import type {
  Tournament,
  TournamentStatus,
  Team,
  Game,
  CrossGameLeaderboardEntry,
  MatchRound,
  WSMessage,
  TournamentGameWithDetails,
} from '../types';

type TabType = 'info' | 'leaderboard' | 'matches' | 'games' | 'teams';

// Icons
const InfoCircleIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />
  </svg>
);

const ChartBarIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z" />
  </svg>
);

const PuzzlePieceIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M14.25 6.087c0-.355.186-.676.401-.959.221-.29.349-.634.349-1.003 0-1.036-1.007-1.875-2.25-1.875s-2.25.84-2.25 1.875c0 .369.128.713.349 1.003.215.283.401.604.401.959v0a.64.64 0 0 1-.657.643 48.39 48.39 0 0 1-4.163-.3c.186 1.613.293 3.25.315 4.907a.656.656 0 0 1-.658.663v0c-.355 0-.676-.186-.959-.401a1.647 1.647 0 0 0-1.003-.349c-1.036 0-1.875 1.007-1.875 2.25s.84 2.25 1.875 2.25c.369 0 .713-.128 1.003-.349.283-.215.604-.401.959-.401v0c.31 0 .555.26.532.57a48.039 48.039 0 0 1-.642 5.056c1.518.19 3.058.309 4.616.354a.64.64 0 0 0 .657-.643v0c0-.355-.186-.676-.401-.959a1.647 1.647 0 0 1-.349-1.003c0-1.035 1.008-1.875 2.25-1.875 1.243 0 2.25.84 2.25 1.875 0 .369-.128.713-.349 1.003-.215.283-.4.604-.4.959v0c0 .333.277.599.61.58a48.1 48.1 0 0 0 5.427-.63 48.05 48.05 0 0 0 .582-4.717.532.532 0 0 0-.533-.57v0c-.355 0-.676.186-.959.401-.29.221-.634.349-1.003.349-1.035 0-1.875-1.007-1.875-2.25s.84-2.25 1.875-2.25c.37 0 .713.128 1.003.349.283.215.604.401.96.401v0a.656.656 0 0 0 .658-.663 48.422 48.422 0 0 0-.37-5.36c-1.886.342-3.81.574-5.766.689a.578.578 0 0 1-.61-.58v0Z" />
  </svg>
);

const UsersIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />
  </svg>
);

const PlayIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 0 1 0 1.972l-11.54 6.347a1.125 1.125 0 0 1-1.667-.986V5.653Z" />
  </svg>
);

const CheckCircleIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
  </svg>
);

const ArrowsExpandIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 3.75v4.5m0-4.5h4.5m-4.5 0L9 9M3.75 20.25v-4.5m0 4.5h4.5m-4.5 0L9 15M20.25 3.75h-4.5m4.5 0v4.5m0-4.5L15 9m5.25 11.25h-4.5m4.5 0v-4.5m0 4.5L15 15" />
  </svg>
);

const XMarkIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
  </svg>
);

const UserPlusIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M18 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0ZM3 19.235v-.11a6.375 6.375 0 0 1 12.75 0v.109A12.318 12.318 0 0 1 9.374 21c-2.331 0-4.512-.645-6.374-1.766Z" />
  </svg>
);

const CalendarIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" />
  </svg>
);

const ClockIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
    <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
  </svg>
);

const ArrowLeftIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18" />
  </svg>
);

const HashtagIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
    <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 8.25h15m-16.5 7.5h15m-1.8-13.5-3.9 19.5m-2.1-19.5-3.9 19.5" />
  </svg>
);

const FolderIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z" />
  </svg>
);

const ChevronDownIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
  </svg>
);

const ChevronRightIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-5 h-5">
    <path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
  </svg>
);

const statusConfig: Record<TournamentStatus, {
  badge: string;
  label: string;
}> = {
  pending: {
    badge: 'badge badge-yellow',
    label: 'Ожидание',
  },
  active: {
    badge: 'badge badge-green',
    label: 'Активный',
  },
  completed: {
    badge: 'badge badge-gray',
    label: 'Завершён',
  },
};

export function TournamentDetail() {
  const { id } = useParams<{ id: string }>();
  const { isAuthenticated, user } = useAuthStore();
  const [tournament, setTournament] = useState<Tournament | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [games, setGames] = useState<Game[]>([]);
  const [crossGameLeaderboard, setCrossGameLeaderboard] = useState<CrossGameLeaderboardEntry[]>([]);
  const [matchRounds, setMatchRounds] = useState<MatchRound[]>([]);
  const [myTeam, setMyTeam] = useState<Team | null>(null);
  const [activeTab, setActiveTab] = useState<TabType>('info');
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [showCrossGameLeaderboard, setShowCrossGameLeaderboard] = useState(true); // По играм / Общий
  const [isRunningMatches, setIsRunningMatches] = useState(false);
  const [isRetryingMatches, setIsRetryingMatches] = useState(false);
  const [isRefreshingMatches, setIsRefreshingMatches] = useState(false);

  // Join modal state
  const [showJoinModal, setShowJoinModal] = useState(false);
  const [teamName, setTeamName] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [isJoining, setIsJoining] = useState(false);

  // Action states
  const [isStarting, setIsStarting] = useState(false);
  const [isCompleting, setIsCompleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  // Games status state (for active game management)
  const [gamesStatus, setGamesStatus] = useState<TournamentGameWithDetails[]>([]);
  const [runningGameId, setRunningGameId] = useState<string | null>(null);
  const [settingActiveGameId, setSettingActiveGameId] = useState<string | null>(null);
  const [resettingGameId, setResettingGameId] = useState<string | null>(null);

  const handleWebSocketMessage = useCallback((message: WSMessage) => {
    // WebSocket updates can trigger refresh of data
    if (message.type === 'leaderboard_update') {
      // Refresh cross-game leaderboard on updates
      if (id) {
        api.getCrossGameLeaderboard(id).then(data => {
          setCrossGameLeaderboard(data || []);
        }).catch(console.error);
      }
    }
  }, [id]);

  // WebSocket for real-time updates (hook handles auth internally)
  const { isConnected } = useWebSocket({
    tournamentId: id || '',
    onMessage: handleWebSocketMessage,
  });

  useEffect(() => {
    if (id) {
      loadTournamentData();
    }
  }, [id]);

  const loadTournamentData = async () => {
    if (!id) return;

    setIsLoading(true);
    setError(null);

    try {
      const tournamentData = await api.getTournament(id);
      setTournament(tournamentData);

      const [teamsData, gamesData, crossGameData, matchRoundsData, gamesStatusData] = await Promise.all([
        api.getTournamentTeams(id).catch(() => []),
        api.getTournamentGames(id).catch(() => []),
        api.getCrossGameLeaderboard(id).catch(() => []),
        api.getMatchesByRounds(id).catch(() => []),
        api.getTournamentGamesStatus(id).catch(() => []),
      ]);

      setTeams(teamsData || []);
      setGames(gamesData || []);
      setCrossGameLeaderboard(crossGameData || []);
      setMatchRounds(matchRoundsData || []);
      setGamesStatus(gamesStatusData || []);

      if (isAuthenticated) {
        try {
          const myTeamData = await api.getMyTeam(id);
          setMyTeam(myTeamData);
        } catch {
          setMyTeam(null);
        }
      }
    } catch (err) {
      setError('Не удалось загрузить данные турнира');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleCreateTeam = async () => {
    if (!id || !teamName.trim()) return;

    setIsJoining(true);
    try {
      const team = await api.createTeam(id, teamName.trim());
      setMyTeam(team);
      setTeams([...teams, team]);
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
    try {
      const team = await api.joinTeamByCode(joinCode.trim());
      setMyTeam(team);
      setShowJoinModal(false);
      setJoinCode('');
      if (id) {
        const teamsData = await api.getTournamentTeams(id);
        setTeams(teamsData || []);
      }
    } catch (err) {
      console.error('Failed to join team:', err);
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
      await loadTournamentData();
    } catch (err: unknown) {
      console.error('Failed to start tournament:', err);
      const errorMessage = err instanceof Error ? err.message : 'Не удалось запустить турнир';
      setActionError(errorMessage);
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
      await loadTournamentData();
    } catch (err: unknown) {
      console.error('Failed to complete tournament:', err);
      const errorMessage = err instanceof Error ? err.message : 'Не удалось завершить турнир';
      setActionError(errorMessage);
    } finally {
      setIsCompleting(false);
    }
  };

  const handleRunAllMatches = async () => {
    if (!tournament) return;

    setIsRunningMatches(true);
    setActionError(null);
    try {
      const result = await api.runAllMatches(tournament.id);
      setActionError(null);
      alert(`Запущено ${result.enqueued} матчей`);
    } catch (err: unknown) {
      console.error('Failed to run matches:', err);
      const errorMessage = err instanceof Error ? err.message : 'Не удалось запустить матчи';
      setActionError(errorMessage);
    } finally {
      setIsRunningMatches(false);
    }
  };

  const handleRetryFailedMatches = async () => {
    if (!tournament) return;

    setIsRetryingMatches(true);
    setActionError(null);
    try {
      const result = await api.retryFailedMatches(tournament.id);
      setActionError(null);
      alert(`Перезапущено ${result.enqueued} неудачных матчей`);
      await loadTournamentData();
    } catch (err: unknown) {
      console.error('Failed to retry matches:', err);
      const errorMessage = err instanceof Error ? err.message : 'Не удалось перезапустить матчи';
      setActionError(errorMessage);
    } finally {
      setIsRetryingMatches(false);
    }
  };

  // Helper function to wait for matches to complete and auto-retry if needed
  const waitForMatchesAndAutoRetry = async (tournamentId: string, initialEnqueued: number) => {
    const MAX_WAIT_TIME = 10 * 60 * 1000; // 10 minutes max
    const POLL_INTERVAL = 2000; // 2 seconds
    const AUTO_RETRY_THRESHOLD = 50;

    const startTime = Date.now();
    let lastPending = initialEnqueued;

    while (Date.now() - startTime < MAX_WAIT_TIME) {
      await new Promise(resolve => setTimeout(resolve, POLL_INTERVAL));

      try {
        const stats = await api.getMatchStatistics(tournamentId);
        const inProgress = stats.pending + stats.running;

        // Refresh leaderboard while matches are running
        if (inProgress !== lastPending) {
          lastPending = inProgress;
          // Refresh data
          const [leaderboardData, matchRoundsData] = await Promise.all([
            api.getCrossGameLeaderboard(tournamentId).catch(() => []),
            api.getMatchesByRounds(tournamentId).catch(() => []),
          ]);
          setCrossGameLeaderboard(leaderboardData);
          setMatchRounds(matchRoundsData || []);
        }

        // All matches completed
        if (inProgress === 0) {
          // Check for failed matches
          if (stats.failed > 0 && stats.failed <= AUTO_RETRY_THRESHOLD) {
            console.log(`Auto-retrying ${stats.failed} failed matches (threshold: ${AUTO_RETRY_THRESHOLD})`);
            try {
              const retryResult = await api.retryFailedMatches(tournamentId);
              if (retryResult.enqueued > 0) {
                // Wait for retry to complete recursively
                await waitForMatchesAndAutoRetry(tournamentId, retryResult.enqueued);
              }
            } catch (retryErr) {
              console.error('Failed to auto-retry matches:', retryErr);
            }
          }
          return;
        }
      } catch (err) {
        console.error('Error polling match status:', err);
      }
    }

    console.warn('Timeout waiting for matches to complete');
  };

  // Run matches for a specific game
  const handleRunGameMatches = async (gameId: string, gameName: string, gameDisplayName: string) => {
    if (!tournament || !id) return;

    setRunningGameId(gameId);
    setActionError(null);
    try {
      const result = await api.runGameMatches(id, gameName);

      // Find current game index and check if there's a next game
      const currentIndex = games.findIndex(g => g.id === gameId);
      const isLastGame = currentIndex === games.length - 1;

      if (!isLastGame) {
        // Switch to the next game
        const nextGame = games[currentIndex + 1];
        await api.setActiveGame(id, nextGame.id);
        alert(`Запущено ${result.enqueued} матчей для "${gameDisplayName}". Активная игра переключена на "${nextGame.display_name}". Ожидание завершения матчей...`);
      } else {
        // Last game - deactivate all games
        await api.deactivateAllGames(id);
        alert(`Запущено ${result.enqueued} матчей для "${gameDisplayName}". Это была последняя игра в турнире. Все игры деактивированы. Ожидание завершения матчей...`);
      }

      // Wait for matches to complete and auto-retry if needed (runs in background)
      waitForMatchesAndAutoRetry(id, result.enqueued).then(() => {
        // Final refresh after all matches complete
        Promise.all([
          api.getTournamentGamesStatus(id).catch(() => []),
          api.getMatchesByRounds(id).catch(() => []),
          api.getCrossGameLeaderboard(id).catch(() => []),
        ]).then(([gamesStatusData, matchRoundsData, leaderboardData]) => {
          setGamesStatus(gamesStatusData || []);
          setMatchRounds(matchRoundsData || []);
          setCrossGameLeaderboard(leaderboardData);
        });
      });

      // Immediate refresh
      const [gamesStatusData, matchRoundsData] = await Promise.all([
        api.getTournamentGamesStatus(id).catch(() => []),
        api.getMatchesByRounds(id).catch(() => []),
      ]);
      setGamesStatus(gamesStatusData || []);
      setMatchRounds(matchRoundsData || []);
    } catch (err: unknown) {
      console.error('Failed to run game matches:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось запустить матчи');
    } finally {
      setRunningGameId(null);
    }
  };

  // Set active game for tournament
  const handleSetActiveGame = async (gameId: string) => {
    if (!id) return;

    setSettingActiveGameId(gameId);
    setActionError(null);
    try {
      await api.setActiveGame(id, gameId);
      // Reload games status
      const gamesStatusData = await api.getTournamentGamesStatus(id);
      setGamesStatus(gamesStatusData || []);
    } catch (err: unknown) {
      console.error('Failed to set active game:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось установить активную игру');
    } finally {
      setSettingActiveGameId(null);
    }
  };

  // Reset game round (delete all matches and reset ratings)
  const handleResetGameRound = async (gameId: string, gameDisplayName: string) => {
    if (!id) return;

    const confirmed = window.confirm(
      `Вы уверены, что хотите сбросить раунд для игры "${gameDisplayName}"?\n\n` +
      'Это действие:\n' +
      '- Удалит все матчи этой игры\n' +
      '- Сбросит рейтинги всех участников до 1000\n' +
      '- Сбросит номер раунда\n\n' +
      'Это действие необратимо!'
    );

    if (!confirmed) return;

    setResettingGameId(gameId);
    setActionError(null);
    try {
      const result = await api.resetGameRound(id, gameId);
      alert(
        `Раунд сброшен успешно!\n\n` +
        `Удалено матчей: ${result.matches_deleted}\n` +
        `Сброшено рейтингов: ${result.participants_reset}\n` +
        `Удалено записей истории: ${result.rating_history_reset}`
      );
      // Reload games status and matches
      const [gamesStatusData, matchRoundsData] = await Promise.all([
        api.getTournamentGamesStatus(id).catch(() => []),
        api.getMatchesByRounds(id).catch(() => []),
      ]);
      setGamesStatus(gamesStatusData || []);
      setMatchRounds(matchRoundsData || []);
    } catch (err: unknown) {
      console.error('Failed to reset game round:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось сбросить раунд');
    } finally {
      setResettingGameId(null);
    }
  };

  // Функция для обновления только матчей (для авто-обновления)
  const refreshMatches = useCallback(async () => {
    if (!id || isRefreshingMatches) return;

    setIsRefreshingMatches(true);
    try {
      const matchRoundsData = await api.getMatchesByRounds(id);
      setMatchRounds(matchRoundsData || []);
    } catch (err) {
      console.error('Failed to refresh matches:', err);
    } finally {
      setIsRefreshingMatches(false);
    }
  }, [id, isRefreshingMatches]);

  // Функция для обновления таблиц рейтинга (для авто-обновления)
  const [isRefreshingLeaderboard, setIsRefreshingLeaderboard] = useState(false);
  const refreshLeaderboard = useCallback(async () => {
    if (!id || isRefreshingLeaderboard) return;

    setIsRefreshingLeaderboard(true);
    try {
      const crossGameData = await api.getCrossGameLeaderboard(id);
      setCrossGameLeaderboard(crossGameData || []);
    } catch (err) {
      console.error('Failed to refresh leaderboard:', err);
    } finally {
      setIsRefreshingLeaderboard(false);
    }
  }, [id, isRefreshingLeaderboard]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-24">
        <div className="text-center">
          <div className="w-12 h-12 border-4 border-primary-200 dark:border-primary-800 border-t-primary-600 dark:border-t-primary-400 rounded-full animate-spin mx-auto mb-4" />
          <p className="text-gray-500 dark:text-gray-200">Загрузка турнира...</p>
        </div>
      </div>
    );
  }

  if (error || !tournament) {
    return (
      <div className="text-center py-24">
        <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-4">
          <XMarkIcon />
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
        <div className="p-6 md:p-10">
          <div className="flex justify-between items-center mb-8">
            <div>
              <h1 className="text-3xl md:text-4xl font-bold mb-2">{tournament.name}</h1>
              <p className="text-gray-400">
                {showCrossGameLeaderboard ? 'Рейтинг по играм' : 'Общий рейтинг'}
              </p>
            </div>
            <div className="flex items-center gap-4">
              <div className="flex gap-2">
                <button
                  onClick={() => setShowCrossGameLeaderboard(true)}
                  className={`btn text-sm ${showCrossGameLeaderboard ? 'bg-primary-600 hover:bg-primary-700' : 'bg-gray-700 hover:bg-gray-600'} text-white`}
                >
                  По играм
                </button>
                <button
                  onClick={() => setShowCrossGameLeaderboard(false)}
                  className={`btn text-sm ${!showCrossGameLeaderboard ? 'bg-primary-600 hover:bg-primary-700' : 'bg-gray-700 hover:bg-gray-600'} text-white`}
                >
                  Общий
                </button>
              </div>
              {isConnected && (
                <span className="online-indicator text-emerald-400">
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
      {/* Header */}
      <div className="mb-8">
        <Link to="/tournaments" className="inline-flex items-center gap-2 text-gray-500 hover:text-primary-600 mb-4 transition-colors">
          <ArrowLeftIcon />
          <span>Назад к турнирам</span>
        </Link>

        <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
          <div>
            <div className="flex flex-wrap items-center gap-3 mb-3">
              <h1 className="text-3xl font-bold text-gray-900 dark:text-gray-100">{tournament.name}</h1>
              <span className={config.badge}>
                {config.label}
              </span>
              {tournament.is_permanent && (
                <span className="badge badge-blue">
                  Постоянный
                </span>
              )}
            </div>
            <div className="flex items-center gap-2 text-gray-600 dark:text-gray-200">
              <HashtagIcon />
              <span>Код:</span>
              <code className="bg-gray-100 dark:bg-gray-800 px-3 py-1 rounded-lg font-mono text-gray-800 dark:text-gray-100">
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
                  onClick={handleRunAllMatches}
                  disabled={isRunningMatches}
                  className="btn btn-primary"
                >
                  <PlayIcon />
                  {isRunningMatches ? 'Запуск...' : 'Запустить раунды'}
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
      <div className="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 mb-6 p-1.5">
        <nav className="flex gap-1 overflow-x-auto">
          {tabs.map((tab) => {
            const TabIcon = tab.icon;
            return (
              <button
                key={tab.id}
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
                      : 'bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-200'
                  }`}>
                    {tab.count}
                  </span>
                )}
              </button>
            );
          })}
        </nav>
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
            hasActiveMatches={matchRounds.some(r => r.pending_count > 0 || r.running_count > 0)}
            isCompleted={tournament.status === 'completed'}
          />
        )}

        {activeTab === 'matches' && (
          <MatchesTab
            rounds={matchRounds}
            onRefresh={refreshMatches}
            isRefreshing={isRefreshingMatches}
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
          />
        )}

        {activeTab === 'teams' && (
          <TeamsTab
            teams={teams}
            isAuthenticated={isAuthenticated}
            myTeam={myTeam}
            tournamentStatus={tournament.status}
            onJoinByCode={handleJoinTeam}
            joinCode={joinCode}
            setJoinCode={setJoinCode}
            isJoining={isJoining}
          />
        )}
      </div>

      {/* Join Modal */}
      {showJoinModal && (
        <div className="modal-backdrop" onClick={() => setShowJoinModal(false)}>
          <div className="modal-content w-full max-w-md p-6 m-4" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100">Участие в турнире</h2>
              <button
                onClick={() => setShowJoinModal(false)}
                className="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors"
              >
                <XMarkIcon />
              </button>
            </div>

            <div className="space-y-6">
              <div>
                <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-3">Создать новую команду</h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={teamName}
                    onChange={(e) => setTeamName(e.target.value)}
                    placeholder="Название команды"
                    className="input flex-1"
                  />
                  <button
                    onClick={handleCreateTeam}
                    disabled={isJoining || !teamName.trim()}
                    className="btn btn-primary"
                  >
                    Создать
                  </button>
                </div>
              </div>

              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <div className="w-full border-t border-gray-200 dark:border-gray-600" />
                </div>
                <div className="relative flex justify-center text-sm">
                  <span className="px-4 bg-white dark:bg-gray-900 text-gray-500 dark:text-gray-200">или</span>
                </div>
              </div>

              <div>
                <h3 className="font-semibold text-gray-900 dark:text-gray-100 mb-3">Присоединиться к существующей</h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={joinCode}
                    onChange={(e) => setJoinCode(e.target.value)}
                    placeholder="Код приглашения"
                    className="input flex-1 font-mono"
                  />
                  <button
                    onClick={handleJoinTeam}
                    disabled={isJoining || !joinCode.trim()}
                    className="btn btn-secondary"
                  >
                    Вступить
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// Info Tab Component
function InfoTab({ tournament }: { tournament: Tournament }) {
  return (
    <div className="card">
      {tournament.description ? (
        <div className="prose max-w-none mb-8">
          <p className="text-gray-700 dark:text-gray-200 leading-relaxed">{tournament.description}</p>
        </div>
      ) : (
        <p className="text-gray-500 dark:text-gray-200 mb-8">Описание не указано.</p>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="stat-card">
          <div className="flex items-center gap-2 text-gray-500 dark:text-gray-200 text-sm mb-1">
            <UsersIcon />
            <span>Макс. размер команды</span>
          </div>
          <p className="text-2xl font-bold text-gray-900 dark:text-gray-100">{tournament.max_team_size}</p>
        </div>

        {tournament.max_participants && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-500 dark:text-gray-200 text-sm mb-1">
              <UsersIcon />
              <span>Макс. участников</span>
            </div>
            <p className="text-2xl font-bold text-gray-900 dark:text-gray-100">{tournament.max_participants}</p>
          </div>
        )}

        {tournament.start_time && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-500 dark:text-gray-200 text-sm mb-1">
              <CalendarIcon />
              <span>Начало</span>
            </div>
            <p className="text-lg font-bold text-gray-900 dark:text-gray-100">
              {new Date(tournament.start_time).toLocaleDateString('ru-RU')}
            </p>
          </div>
        )}

        {tournament.end_time && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-500 dark:text-gray-200 text-sm mb-1">
              <CalendarIcon />
              <span>Окончание</span>
            </div>
            <p className="text-lg font-bold text-gray-900 dark:text-gray-100">
              {new Date(tournament.end_time).toLocaleDateString('ru-RU')}
            </p>
          </div>
        )}

        <div className="stat-card">
          <div className="flex items-center gap-2 text-gray-500 dark:text-gray-200 text-sm mb-1">
            <ClockIcon />
            <span>Создан</span>
          </div>
          <p className="text-lg font-bold text-gray-900 dark:text-gray-100">
            {new Date(tournament.created_at).toLocaleDateString('ru-RU')}
          </p>
        </div>
      </div>
    </div>
  );
}

// Animated Podium Component for Tournament Winners
function WinnersPodium({ entries }: { entries: CrossGameLeaderboardEntry[] }) {
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    // Trigger animation after mount
    const timer = setTimeout(() => setIsVisible(true), 100);
    return () => clearTimeout(timer);
  }, []);

  if (entries.length < 3) return null;

  const first = entries[0];
  const second = entries[1];
  const third = entries[2];

  const podiumData = [
    { entry: second, place: 2, height: 'h-28', delay: 'delay-300', bgGradient: 'from-gray-300 via-gray-200 to-gray-400', textColor: 'text-gray-700', medal: '🥈' },
    { entry: first, place: 1, height: 'h-40', delay: 'delay-100', bgGradient: 'from-yellow-400 via-amber-300 to-yellow-500', textColor: 'text-amber-900', medal: '🥇' },
    { entry: third, place: 3, height: 'h-20', delay: 'delay-500', bgGradient: 'from-orange-400 via-orange-300 to-orange-500', textColor: 'text-orange-900', medal: '🥉' },
  ];

  return (
    <div className="mb-8 p-6 bg-gradient-to-b from-primary-900/20 via-primary-800/10 to-transparent dark:from-primary-900/30 dark:via-primary-800/20 rounded-2xl">
      <div className="text-center mb-6">
        <h3 className="text-2xl font-bold text-gray-900 dark:text-gray-100 mb-1">
          🏆 Победители турнира 🏆
        </h3>
        <p className="text-gray-600 dark:text-gray-400">Поздравляем финалистов!</p>
      </div>

      <div className="flex items-end justify-center gap-4 max-w-2xl mx-auto">
        {podiumData.map(({ entry, place, height, delay, bgGradient, textColor, medal }) => (
          <div
            key={place}
            className={`flex-1 max-w-48 transition-all duration-700 ease-out ${
              isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-10'
            } ${delay}`}
          >
            {/* Winner card */}
            <div className={`text-center mb-2 transform transition-all duration-500 ${
              isVisible ? 'scale-100' : 'scale-0'
            } ${delay}`}>
              <div className="text-4xl mb-2 animate-bounce" style={{ animationDelay: `${(place - 1) * 200}ms`, animationDuration: '2s' }}>
                {medal}
              </div>
              <div className="font-bold text-lg text-gray-900 dark:text-gray-100 truncate px-2">
                {entry.team_name || entry.program_name}
              </div>
              <div className="text-2xl font-bold bg-gradient-to-r from-primary-600 to-primary-400 bg-clip-text text-transparent">
                {entry.total_rating.toLocaleString()}
              </div>
              <div className="text-xs text-gray-500 dark:text-gray-400">
                {entry.total_wins}W / {entry.total_losses}L
              </div>
            </div>

            {/* Podium */}
            <div
              className={`${height} bg-gradient-to-t ${bgGradient} rounded-t-lg shadow-lg relative overflow-hidden transition-all duration-700 ease-out ${
                isVisible ? 'opacity-100' : 'opacity-0'
              } ${delay}`}
            >
              <div className="absolute inset-0 bg-white/20 animate-pulse" style={{ animationDuration: '3s' }} />
              <div className={`absolute inset-x-0 bottom-0 flex items-center justify-center pb-2 ${textColor}`}>
                <span className="text-3xl font-black">{place}</span>
              </div>
              {/* Shine effect */}
              <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent -translate-x-full animate-shine" />
            </div>
          </div>
        ))}
      </div>

      {/* Confetti-like decorative elements */}
      <div className="relative h-8 overflow-hidden">
        {[...Array(20)].map((_, i) => (
          <div
            key={i}
            className={`absolute w-2 h-2 rounded-full animate-confetti`}
            style={{
              left: `${5 + i * 5}%`,
              backgroundColor: ['#fbbf24', '#a3a3a3', '#fb923c', '#22c55e', '#3b82f6'][i % 5],
              animationDelay: `${i * 0.1}s`,
              animationDuration: `${2 + Math.random()}s`,
            }}
          />
        ))}
      </div>
    </div>
  );
}

// Leaderboard Tab Component
function LeaderboardTab({
  crossGameEntries,
  games,
  isConnected,
  showCrossGame,
  onShowCrossGameChange,
  onToggleFullscreen,
  onRefresh,
  isRefreshing,
  hasActiveMatches,
  isCompleted,
}: {
  crossGameEntries: CrossGameLeaderboardEntry[];
  games: Game[];
  isConnected: boolean;
  showCrossGame: boolean;
  onShowCrossGameChange: (value: boolean) => void;
  onToggleFullscreen: () => void;
  onRefresh: () => void;
  isRefreshing: boolean;
  hasActiveMatches: boolean;
  isCompleted: boolean;
}) {
  const [autoRefresh, setAutoRefresh] = useState(true);

  // Автообновление каждые 2 секунды если есть активные матчи
  useEffect(() => {
    if (!autoRefresh || !hasActiveMatches) return;

    const interval = setInterval(() => {
      onRefresh();
    }, 2000);

    return () => clearInterval(interval);
  }, [autoRefresh, hasActiveMatches, onRefresh]);

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100">Рейтинг</h2>
          {isConnected && (
            <span className="online-indicator">
              Онлайн
            </span>
          )}
          {hasActiveMatches && autoRefresh && (
            <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 text-xs">
              <span className="w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
              Обновление...
            </span>
          )}
          {isRefreshing && (
            <div className="w-4 h-4 border-2 border-primary-200 dark:border-primary-800 border-t-primary-600 dark:border-t-primary-400 rounded-full animate-spin" />
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {hasActiveMatches && (
            <button
              onClick={() => setAutoRefresh(!autoRefresh)}
              className={`btn text-sm ${autoRefresh ? 'btn-primary' : 'btn-secondary'}`}
            >
              {autoRefresh ? 'Авто-обновление вкл' : 'Авто-обновление выкл'}
            </button>
          )}
          <button
            onClick={onRefresh}
            disabled={isRefreshing}
            className="btn btn-secondary text-sm"
          >
            Обновить
          </button>
          <button
            onClick={() => onShowCrossGameChange(true)}
            className={`btn ${showCrossGame ? 'btn-primary' : 'btn-secondary'}`}
          >
            По играм
          </button>
          <button
            onClick={() => onShowCrossGameChange(false)}
            className={`btn ${!showCrossGame ? 'btn-primary' : 'btn-secondary'}`}
          >
            Общий
          </button>
          <button onClick={onToggleFullscreen} className="btn btn-secondary">
            <ArrowsExpandIcon />
            На весь экран
          </button>
        </div>
      </div>

      {/* Show animated podium for completed tournaments */}
      {isCompleted && crossGameEntries.length >= 3 && (
        <WinnersPodium entries={crossGameEntries} />
      )}

      {showCrossGame ? (
        <CrossGameLeaderboardTable entries={crossGameEntries} games={games} />
      ) : (
        <GeneralLeaderboardTable entries={crossGameEntries} />
      )}
    </div>
  );
}

// General Leaderboard Table Component - uses CrossGameLeaderboardEntry data
// Shows: rank, team name, total score, games played, score per game
function GeneralLeaderboardTable({
  entries,
  isDark = false,
}: {
  entries: CrossGameLeaderboardEntry[];
  isDark?: boolean;
}) {
  if (entries.length === 0) {
    return (
      <div className={`empty-state ${isDark ? 'text-gray-400' : ''}`}>
        <div className="empty-state-icon">
          <ChartBarIcon />
        </div>
        <h3 className="empty-state-title">Пока нет результатов</h3>
        <p className="empty-state-description">
          Таблица обновится после завершения матчей
        </p>
      </div>
    );
  }

  // Find max score for visual bars
  const maxScore = Math.max(...entries.map(e => e.total_rating), 1);

  const getRankBadge = (rank: number) => {
    if (rank === 1) {
      return (
        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-yellow-300 to-amber-500 flex items-center justify-center shadow-lg shadow-amber-500/30">
          <svg className="w-5 h-5 text-amber-900" fill="currentColor" viewBox="0 0 20 20">
            <path fillRule="evenodd" d="M5 5V.13a2.96 2.96 0 0 0-1.293.749L.879 3.707A2.96 2.96 0 0 0 .13 5H5Zm1.5 6.5H2a2 2 0 0 0-2 2v3a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2v-3a2 2 0 0 0-2-2H6.5ZM6 9a2 2 0 1 0 0-4 2 2 0 0 0 0 4Zm7.5 2.5H18a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2h-4.5v-7Zm1.5-6a2 2 0 1 0 0 4 2 2 0 0 0 0-4Z" clipRule="evenodd"/>
          </svg>
        </div>
      );
    }
    if (rank === 2) {
      return (
        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-gray-200 to-gray-400 flex items-center justify-center shadow-lg shadow-gray-500/20">
          <span className="font-bold text-gray-700">2</span>
        </div>
      );
    }
    if (rank === 3) {
      return (
        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-orange-300 to-orange-500 flex items-center justify-center shadow-lg shadow-orange-500/20">
          <span className="font-bold text-orange-900">3</span>
        </div>
      );
    }
    return (
      <div className={`w-10 h-10 rounded-full flex items-center justify-center font-bold ${
        isDark ? 'bg-gray-700 text-gray-300' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'
      }`}>
        {rank}
      </div>
    );
  };

  const getRowClass = (index: number) => {
    if (index === 0) return isDark ? 'bg-amber-900/10' : 'bg-amber-50/50 dark:bg-amber-900/10';
    if (index === 1) return isDark ? 'bg-gray-700/20' : 'bg-gray-50/50 dark:bg-gray-700/20';
    if (index === 2) return isDark ? 'bg-orange-900/10' : 'bg-orange-50/50 dark:bg-orange-900/10';
    return '';
  };

  return (
    <div className={`space-y-2 ${isDark ? '' : ''}`}>
      {/* Card-style entries */}
      {entries.map((entry, index) => (
        <div
          key={entry.program_id}
          className={`p-4 rounded-xl transition-all ${
            isDark
              ? `bg-gray-800/50 border border-gray-700 ${getRowClass(index)}`
              : `bg-white dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700 ${getRowClass(index)} hover:shadow-md`
          }`}
        >
          <div className="flex items-center gap-4">
            {/* Rank */}
            {getRankBadge(entry.rank)}

            {/* Team Info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between gap-4">
                <div className="min-w-0">
                  <h3 className={`font-bold text-lg truncate ${isDark ? 'text-white' : 'text-gray-900 dark:text-gray-100'}`}>
                    {entry.team_name || entry.program_name}
                  </h3>
                  <div className={`flex items-center gap-3 text-sm ${isDark ? 'text-gray-400' : 'text-gray-500 dark:text-gray-400'}`}>
                    <span>{entry.total_games} игр</span>
                    <span>•</span>
                    <span className="text-emerald-600 dark:text-emerald-400">{entry.total_wins}W</span>
                    <span className="text-red-600 dark:text-red-400">{entry.total_losses}L</span>
                  </div>
                </div>

                {/* Score */}
                <div className="text-right shrink-0">
                  <div className={`text-3xl font-bold tabular-nums ${
                    index === 0 ? 'text-amber-500' :
                    index === 1 ? 'text-gray-500 dark:text-gray-400' :
                    index === 2 ? 'text-orange-500' :
                    isDark ? 'text-primary-400' : 'text-primary-600 dark:text-primary-400'
                  }`}>
                    {entry.total_rating.toLocaleString()}
                  </div>
                  <div className={`text-xs ${isDark ? 'text-gray-500' : 'text-gray-400 dark:text-gray-500'}`}>
                    очков
                  </div>
                </div>
              </div>

              {/* Score bar */}
              <div className="mt-3 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all duration-500 ${
                    index === 0 ? 'bg-gradient-to-r from-amber-400 to-amber-500' :
                    index === 1 ? 'bg-gradient-to-r from-gray-300 to-gray-400 dark:from-gray-500 dark:to-gray-600' :
                    index === 2 ? 'bg-gradient-to-r from-orange-400 to-orange-500' :
                    'bg-gradient-to-r from-primary-400 to-primary-500'
                  }`}
                  style={{ width: `${(entry.total_rating / maxScore) * 100}%` }}
                />
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

// Cross-Game Leaderboard Table Component
function CrossGameLeaderboardTable({
  entries,
  games,
  isDark = false,
}: {
  entries: CrossGameLeaderboardEntry[];
  games: Game[];
  isDark?: boolean;
}) {
  if (entries.length === 0) {
    return (
      <div className={`empty-state ${isDark ? 'text-gray-400' : ''}`}>
        <div className="empty-state-icon">
          <ChartBarIcon />
        </div>
        <h3 className="empty-state-title">Пока нет результатов</h3>
        <p className="empty-state-description">
          Таблица обновится после завершения матчей
        </p>
      </div>
    );
  }

  const getRankClass = (index: number) => {
    if (index === 0) return 'rank-badge rank-gold';
    if (index === 1) return 'rank-badge rank-silver';
    if (index === 2) return 'rank-badge rank-bronze';
    return isDark ? 'rank-badge bg-gray-700 text-gray-300' : 'rank-badge rank-default';
  };

  const getRowClass = (index: number) => {
    if (index === 0) return isDark ? 'bg-amber-900/20' : 'leaderboard-row-gold';
    if (index === 1) return isDark ? 'bg-gray-700/30' : 'leaderboard-row-silver';
    if (index === 2) return isDark ? 'bg-orange-900/20' : 'leaderboard-row-bronze';
    return '';
  };

  return (
    <div className={`overflow-x-auto ${isDark ? '' : 'card p-0'}`}>
      <table className={`w-full ${isDark ? 'text-white' : 'dark:text-gray-100'}`}>
        <thead className={isDark ? 'bg-gray-800/50' : 'bg-gray-50 dark:bg-gray-800/50'}>
          <tr>
            <th className="px-4 py-3 text-left font-semibold text-sm uppercase tracking-wide">Место</th>
            <th className="px-4 py-3 text-left font-semibold text-sm uppercase tracking-wide">Команда</th>
            {games.map((game) => (
              <th key={game.id} className="px-4 py-3 text-center font-semibold text-sm uppercase tracking-wide">
                {game.display_name}
              </th>
            ))}
            <th className="px-4 py-3 text-right font-semibold text-sm uppercase tracking-wide">Сумма</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry, index) => (
            <tr
              key={entry.program_id}
              className={`border-b ${isDark ? 'border-gray-700' : 'border-gray-100 dark:border-gray-700'} ${getRowClass(index)} transition-colors`}
            >
              <td className="px-4 py-3">
                <span className={getRankClass(index)}>
                  {entry.rank}
                </span>
              </td>
              <td className="px-4 py-3">
                <span className="font-semibold">
                  {entry.team_name || entry.program_name}
                </span>
              </td>
              {games.map((game) => {
                const gameRating = entry.game_ratings[game.id];
                return (
                  <td key={game.id} className="px-4 py-3 text-center">
                    {gameRating ? (
                      <div>
                        <span className="font-mono font-bold">{Math.round(gameRating.rating)}</span>
                        <div className={`text-xs ${isDark ? 'text-gray-400' : 'text-gray-500 dark:text-gray-200'}`}>
                          <span className="text-emerald-500" title="Побед">{gameRating.wins}</span>
                          <span className="mx-0.5">/</span>
                          <span className="text-red-500" title="Поражений">{gameRating.losses}</span>
                          <span className="mx-0.5">/</span>
                          <span title="Ничьих">{gameRating.draws || 0}</span>
                        </div>
                      </div>
                    ) : (
                      <span className="text-gray-400">-</span>
                    )}
                  </td>
                );
              })}
              <td className="px-4 py-3 text-right">
                <span className={`font-mono font-bold text-lg ${isDark ? 'text-primary-400' : 'text-primary-600 dark:text-primary-400'}`}>
                  {entry.total_rating}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Dark mode alias for fullscreen
function CrossGameLeaderboardTableDark({
  entries,
  games,
}: {
  entries: CrossGameLeaderboardEntry[];
  games: Game[];
}) {
  return <CrossGameLeaderboardTable entries={entries} games={games} isDark />;
}

// Games Tab Component
function GamesTab({
  games,
  gamesStatus,
  tournamentId,
  myTeam,
  isAdmin,
  tournamentStatus,
  onRunGameMatches,
  onSetActiveGame,
  onResetGameRound,
  runningGameId,
  settingActiveGameId,
  resettingGameId,
}: {
  games: Game[];
  gamesStatus: TournamentGameWithDetails[];
  tournamentId: string;
  myTeam: Team | null;
  isAdmin?: boolean;
  tournamentStatus?: TournamentStatus;
  onRunGameMatches?: (gameId: string, gameName: string, gameDisplayName: string) => Promise<void>;
  onSetActiveGame?: (gameId: string) => Promise<void>;
  onResetGameRound?: (gameId: string, gameDisplayName: string) => Promise<void>;
  runningGameId?: string | null;
  settingActiveGameId?: string | null;
  resettingGameId?: string | null;
}) {
  const handleRunMatches = async (e: React.MouseEvent, game: Game) => {
    e.preventDefault();
    e.stopPropagation();
    if (!onRunGameMatches) return;
    await onRunGameMatches(game.id, game.name, game.display_name);
  };

  const handleSetActive = async (e: React.MouseEvent, gameId: string) => {
    e.preventDefault();
    e.stopPropagation();
    if (!onSetActiveGame) return;
    await onSetActiveGame(gameId);
  };

  const handleReset = async (e: React.MouseEvent, game: Game) => {
    e.preventDefault();
    e.stopPropagation();
    if (!onResetGameRound) return;
    await onResetGameRound(game.id, game.display_name);
  };

  if (games.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-state-icon">
          <PuzzlePieceIcon />
        </div>
        <h3 className="empty-state-title">Нет игр</h3>
        <p className="empty-state-description">
          В этот турнир еще не добавлены игры
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Admin info banner */}
      {isAdmin && tournamentStatus === 'active' && (
        <div className="mb-6 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-700 rounded-lg">
          <div className="flex items-start gap-3">
            <div className="shrink-0 w-8 h-8 bg-blue-100 dark:bg-blue-800 rounded-lg flex items-center justify-center">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-blue-600 dark:text-blue-400">
                <path strokeLinecap="round" strokeLinejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />
              </svg>
            </div>
            <div>
              <h4 className="font-medium text-blue-900 dark:text-blue-200">Режим администратора</h4>
              <p className="text-sm text-blue-700 dark:text-blue-300 mt-1">
                Выберите активную игру и запустите раунд матчей. После запуска матчей команды не смогут изменять свои программы для этой игры.
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {games.map((game, index) => {
          const gameStatus = gamesStatus.find(g => g.game_id === game.id);
          const isActive = gameStatus?.is_active || false;
          const currentRound = gameStatus?.current_round || 0;

          return (
            <Link
              key={game.id}
              to={`/tournaments/${tournamentId}/games/${game.id}`}
              className={`card card-interactive group relative overflow-hidden ${
                isActive ? 'ring-2 ring-green-500 dark:ring-green-600' : ''
              }`}
            >
              {/* Game number badge */}
              <div className="absolute top-3 right-3 w-8 h-8 bg-gray-100 dark:bg-gray-700 rounded-full flex items-center justify-center text-sm font-bold text-gray-600 dark:text-gray-300">
                {index + 1}
              </div>

              <div className="flex items-start justify-between mb-3 pr-10">
                <div>
                  <div className="flex items-center gap-2">
                    <h3 className="text-lg font-bold text-gray-900 dark:text-gray-100 group-hover:text-primary-600 dark:group-hover:text-primary-400 transition-colors">
                      {game.display_name}
                    </h3>
                    {isActive && (
                      <span className="px-2 py-0.5 bg-green-100 dark:bg-green-900/50 text-green-700 dark:text-green-400 text-xs rounded-full font-medium">
                        Активна
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2 mt-1">
                    <code className="text-sm bg-gray-100 dark:bg-gray-700 px-2 py-0.5 rounded text-gray-500 dark:text-gray-200">
                      {game.name}
                    </code>
                    {currentRound > 0 && (
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        • Раунд {currentRound}
                      </span>
                    )}
                  </div>
                </div>
              </div>

              {game.rules && (
                <p className="text-gray-600 dark:text-gray-200 text-sm line-clamp-3 mb-4">
                  {game.rules.substring(0, 200)}...
                </p>
              )}

              <div className="flex items-center justify-between pt-3 border-t border-gray-100 dark:border-gray-700">
                {myTeam && !isAdmin && (
                  <div className="flex items-center gap-2 text-primary-600 dark:text-primary-400 text-sm font-medium">
                    <PlayIcon />
                    <span>Управление программой</span>
                  </div>
                )}

                {/* Admin controls */}
                {isAdmin && tournamentStatus === 'active' && (
                  <div className="flex items-center gap-2 flex-wrap">
                    {!isActive ? (
                      <button
                        onClick={(e) => handleSetActive(e, game.id)}
                        disabled={settingActiveGameId === game.id}
                        className="btn btn-secondary text-xs py-1.5 px-3"
                      >
                        {settingActiveGameId === game.id ? (
                          <>
                            <span className="w-3 h-3 border-2 border-gray-400/30 border-t-gray-600 rounded-full animate-spin" />
                            Установка...
                          </>
                        ) : (
                          'Сделать активной'
                        )}
                      </button>
                    ) : (
                      <>
                        <button
                          onClick={(e) => handleRunMatches(e, game)}
                          disabled={runningGameId === game.id}
                          className="btn btn-primary text-xs py-1.5 px-3"
                        >
                          {runningGameId === game.id ? (
                            <>
                              <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                              Запуск...
                            </>
                          ) : (
                            <>
                              <PlayIcon />
                              Запустить раунд
                            </>
                          )}
                        </button>
                        <button
                          onClick={(e) => handleReset(e, game)}
                          disabled={resettingGameId === game.id}
                          className="btn text-xs py-1.5 px-3 bg-red-600 hover:bg-red-700 text-white"
                          title="Сбросить раунд (удалить все матчи и рейтинги)"
                        >
                          {resettingGameId === game.id ? 'Сброс...' : 'Сбросить'}
                        </button>
                      </>
                    )}
                  </div>
                )}
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}

// Teams Tab Component
function TeamsTab({
  teams,
  isAuthenticated,
  myTeam,
  tournamentStatus,
  onJoinByCode,
  joinCode,
  setJoinCode,
  isJoining,
}: {
  teams: Team[];
  isAuthenticated: boolean;
  myTeam: Team | null;
  tournamentStatus: TournamentStatus;
  onJoinByCode: () => void;
  joinCode: string;
  setJoinCode: (code: string) => void;
  isJoining: boolean;
}) {
  const showJoinSection = isAuthenticated && !myTeam && tournamentStatus === 'pending';

  return (
    <div>
      {/* Join by code section */}
      {showJoinSection && (
        <div className="card mb-6 bg-blue-50 dark:bg-blue-900/30 border-blue-200 dark:border-blue-700">
          <h3 className="font-semibold mb-3 text-blue-900 dark:text-blue-200">Присоединиться к команде</h3>
          <p className="text-sm text-blue-700 dark:text-blue-300 mb-3">
            Введите код приглашения, полученный от капитана команды
          </p>
          <div className="flex gap-2">
            <input
              type="text"
              value={joinCode}
              onChange={(e) => setJoinCode(e.target.value.toUpperCase())}
              placeholder="Код приглашения (например: ABC123)"
              className="input flex-1 uppercase tracking-wider"
              maxLength={10}
            />
            <button
              onClick={onJoinByCode}
              disabled={isJoining || !joinCode.trim()}
              className="btn btn-primary"
            >
              {isJoining ? 'Вступление...' : 'Вступить'}
            </button>
          </div>
        </div>
      )}

      {/* Teams list */}
      {teams.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon">
            <UsersIcon />
          </div>
          <h3 className="empty-state-title">Нет команд</h3>
          <p className="empty-state-description">
            Ни одна команда еще не присоединилась к турниру
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {teams.map((team, index) => (
            <div
              key={team.id}
              className={`card group hover:shadow-lg dark:hover:shadow-gray-900/50 transition-shadow ${
                myTeam?.id === team.id
                  ? 'border-2 border-primary-500 bg-primary-50/50 dark:bg-primary-900/20'
                  : ''
              }`}
            >
              <div className="flex items-center gap-3">
                <div className={`w-10 h-10 rounded-lg flex items-center justify-center text-white font-bold ${
                  myTeam?.id === team.id ? 'bg-primary-600 dark:bg-primary-500' : 'bg-gray-500'
                }`}>
                  {index + 1}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <h3 className="font-semibold text-gray-900 dark:text-gray-100 truncate">{team.name}</h3>
                    {myTeam?.id === team.id && (
                      <span className="badge badge-blue text-xs">Ваша</span>
                    )}
                  </div>
                  <p className="text-sm text-gray-500 dark:text-gray-200">
                    {new Date(team.created_at).toLocaleDateString('ru-RU')}
                  </p>
                </div>
              </div>

              {myTeam?.id === team.id && (
                <Link
                  to={`/teams/${team.id}`}
                  className="mt-3 inline-flex items-center gap-1 text-primary-600 dark:text-primary-400 hover:text-primary-800 dark:hover:text-primary-300 text-sm font-medium"
                >
                  Управление командой
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-4 h-4">
                    <path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
                  </svg>
                </Link>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

// Matches Tab Component - отображает матчи, сгруппированные по раундам
function MatchesTab({
  rounds,
  onRefresh,
  isRefreshing
}: {
  rounds: MatchRound[];
  onRefresh: () => void;
  isRefreshing: boolean;
}) {
  const [expandedRounds, setExpandedRounds] = useState<Set<number>>(new Set());
  const [autoRefresh, setAutoRefresh] = useState(true);

  // Проверяем, есть ли активные матчи (pending или running)
  const hasActiveMatches = rounds.some(
    r => r.pending_count > 0 || r.running_count > 0
  );

  // Автообновление каждые 2 секунды если есть активные матчи
  useEffect(() => {
    if (!autoRefresh || !hasActiveMatches) return;

    const interval = setInterval(() => {
      onRefresh();
    }, 2000);

    return () => clearInterval(interval);
  }, [autoRefresh, hasActiveMatches, onRefresh]);

  const toggleRound = (roundNumber: number) => {
    setExpandedRounds(prev => {
      const next = new Set(prev);
      if (next.has(roundNumber)) {
        next.delete(roundNumber);
      } else {
        next.add(roundNumber);
      }
      return next;
    });
  };

  const expandAll = () => {
    setExpandedRounds(new Set(rounds.map(r => r.round_number)));
  };

  const collapseAll = () => {
    setExpandedRounds(new Set());
  };

  if (rounds.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-state-icon">
          <FolderIcon />
        </div>
        <h3 className="empty-state-title">Нет матчей</h3>
        <p className="empty-state-description">
          Матчи появятся после запуска раундов
        </p>
      </div>
    );
  }

  // Суммарная статистика по всем раундам
  const totalStats = rounds.reduce(
    (acc, round) => ({
      total: acc.total + round.total_matches,
      completed: acc.completed + round.completed_count,
      pending: acc.pending + round.pending_count,
      running: acc.running + round.running_count,
      failed: acc.failed + round.failed_count,
    }),
    { total: 0, completed: 0, pending: 0, running: 0, failed: 0 }
  );

  return (
    <div>
      {/* Header with summary stats */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100">
              Матчи по раундам
            </h2>
            {hasActiveMatches && autoRefresh && (
              <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 text-xs">
                <span className="w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
                Обновление...
              </span>
            )}
            {isRefreshing && (
              <div className="w-4 h-4 border-2 border-primary-200 dark:border-primary-800 border-t-primary-600 dark:border-t-primary-400 rounded-full animate-spin" />
            )}
          </div>
          <div className="flex flex-wrap gap-3 text-sm">
            <span className="text-gray-600 dark:text-gray-200">
              Всего: <strong className="text-gray-900 dark:text-gray-100">{totalStats.total}</strong>
            </span>
            <span className="text-emerald-600 dark:text-emerald-400">
              Завершено: <strong>{totalStats.completed}</strong>
            </span>
            {totalStats.running > 0 && (
              <span className="text-blue-600 dark:text-blue-400">
                Выполняется: <strong>{totalStats.running}</strong>
              </span>
            )}
            {totalStats.pending > 0 && (
              <span className="text-yellow-600 dark:text-yellow-400">
                В очереди: <strong>{totalStats.pending}</strong>
              </span>
            )}
            {totalStats.failed > 0 && (
              <span className="text-red-600 dark:text-red-400">
                Ошибки: <strong>{totalStats.failed}</strong>
              </span>
            )}
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {hasActiveMatches && (
            <button
              onClick={() => setAutoRefresh(!autoRefresh)}
              className={`btn text-sm ${autoRefresh ? 'btn-primary' : 'btn-secondary'}`}
            >
              {autoRefresh ? 'Авто-обновление вкл' : 'Авто-обновление выкл'}
            </button>
          )}
          <button
            onClick={onRefresh}
            disabled={isRefreshing}
            className="btn btn-secondary text-sm"
          >
            Обновить
          </button>
          <button onClick={expandAll} className="btn btn-secondary text-sm">
            Развернуть все
          </button>
          <button onClick={collapseAll} className="btn btn-secondary text-sm">
            Свернуть все
          </button>
        </div>
      </div>

      {/* Overall progress bar */}
      {totalStats.total > 0 && (
        <div className="mb-6 card p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-700 dark:text-gray-200">
              Общий прогресс
            </span>
            <span className="text-sm font-mono text-gray-600 dark:text-gray-200">
              {totalStats.completed} / {totalStats.total} ({Math.round((totalStats.completed / totalStats.total) * 100)}%)
            </span>
          </div>
          <div className="w-full h-4 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
            <div className="h-full flex">
              {/* Completed - green */}
              <div
                className="bg-emerald-500 transition-all duration-500"
                style={{ width: `${(totalStats.completed / totalStats.total) * 100}%` }}
              />
              {/* Running - blue, animated */}
              <div
                className="bg-blue-500 animate-pulse transition-all duration-500"
                style={{ width: `${(totalStats.running / totalStats.total) * 100}%` }}
              />
              {/* Failed - red */}
              <div
                className="bg-red-500 transition-all duration-500"
                style={{ width: `${(totalStats.failed / totalStats.total) * 100}%` }}
              />
            </div>
          </div>
          <div className="flex flex-wrap gap-4 mt-2 text-xs">
            <span className="flex items-center gap-1">
              <span className="w-3 h-3 rounded-full bg-emerald-500" />
              Завершено
            </span>
            {totalStats.running > 0 && (
              <span className="flex items-center gap-1">
                <span className="w-3 h-3 rounded-full bg-blue-500 animate-pulse" />
                Выполняется
              </span>
            )}
            {totalStats.pending > 0 && (
              <span className="flex items-center gap-1">
                <span className="w-3 h-3 rounded-full bg-gray-300 dark:bg-gray-600" />
                В очереди
              </span>
            )}
            {totalStats.failed > 0 && (
              <span className="flex items-center gap-1">
                <span className="w-3 h-3 rounded-full bg-red-500" />
                Ошибки
              </span>
            )}
          </div>
        </div>
      )}

      {/* Rounds list */}
      <div className="space-y-3">
        {rounds.map((round) => (
          <RoundCard
            key={`${round.round_number}-${round.game_type}`}
            round={round}
            isExpanded={expandedRounds.has(round.round_number)}
            onToggle={() => toggleRound(round.round_number)}
          />
        ))}
      </div>
    </div>
  );
}

// Game name display mapping
const gameDisplayNames: Record<string, string> = {
  dilemma: 'Дилемма заключённого',
  tug_of_war: 'Перетягивание каната',
};

const getGameDisplayName = (gameType: string) => gameDisplayNames[gameType] || gameType;

// Компонент карточки раунда
function RoundCard({
  round,
  isExpanded,
  onToggle,
}: {
  round: MatchRound;
  isExpanded: boolean;
  onToggle: () => void;
}) {
  const getStatusColor = () => {
    if (round.failed_count > 0) return 'border-l-red-500';
    if (round.running_count > 0) return 'border-l-blue-500';
    if (round.pending_count > 0) return 'border-l-yellow-500';
    if (round.completed_count === round.total_matches) return 'border-l-emerald-500';
    return 'border-l-gray-300 dark:border-l-gray-600';
  };

  const getProgressPercent = () => {
    if (round.total_matches === 0) return 0;
    return Math.round((round.completed_count / round.total_matches) * 100);
  };

  // Подсчёт статистики по победам/ничьим
  const matchStats = round.matches.reduce(
    (acc, match) => {
      if (match.status === 'completed') {
        if (match.winner === 1) acc.wins1++;
        else if (match.winner === 2) acc.wins2++;
        else acc.draws++;
      }
      return acc;
    },
    { wins1: 0, wins2: 0, draws: 0 }
  );

  return (
    <div className={`card p-0 border-l-4 ${getStatusColor()} overflow-hidden`}>
      {/* Round header - collapsible */}
      <button
        onClick={onToggle}
        className="w-full px-4 py-3 flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-800/50 transition-colors"
      >
        <div className="flex items-center gap-3">
          <div className="text-gray-500 dark:text-gray-200">
            {isExpanded ? <ChevronDownIcon /> : <ChevronRightIcon />}
          </div>
          <div className="flex items-center gap-2">
            <FolderIcon />
            <span className="font-semibold text-gray-900 dark:text-gray-100">
              Раунд {round.round_number}
            </span>
            <span className="px-2 py-0.5 bg-primary-100 dark:bg-primary-900/30 text-primary-700 dark:text-primary-400 text-xs rounded-full font-medium">
              {getGameDisplayName(round.game_type)}
            </span>
          </div>
          <span className="text-sm text-gray-500 dark:text-gray-200">
            {round.total_matches} матчей
          </span>
        </div>

        <div className="flex items-center gap-4">
          {/* Mini stats badges */}
          <div className="hidden sm:flex items-center gap-2 text-xs">
            {round.completed_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400">
                {round.completed_count} завершено
              </span>
            )}
            {round.running_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400">
                {round.running_count} выполняется
              </span>
            )}
            {round.pending_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400">
                {round.pending_count} в очереди
              </span>
            )}
            {round.failed_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400">
                {round.failed_count} ошибок
              </span>
            )}
          </div>

          {/* Progress bar */}
          <div className="w-24 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-emerald-500 transition-all duration-300"
              style={{ width: `${getProgressPercent()}%` }}
            />
          </div>
          <span className="text-sm font-mono text-gray-600 dark:text-gray-200 w-12 text-right">
            {getProgressPercent()}%
          </span>
        </div>
      </button>

      {/* Expanded content */}
      {isExpanded && (
        <div className="border-t border-gray-200 dark:border-gray-700">
          {/* Round summary */}
          <div className="px-4 py-3 bg-gray-50 dark:bg-gray-800/30 flex flex-wrap gap-4 text-sm">
            <span className="text-gray-600 dark:text-gray-200">
              Дата: <strong className="text-gray-900 dark:text-gray-100">
                {new Date(round.created_at).toLocaleString('ru-RU')}
              </strong>
            </span>
            {round.completed_count > 0 && (
              <>
                <span className="text-emerald-600 dark:text-emerald-400">
                  Побед P1: <strong>{matchStats.wins1}</strong>
                </span>
                <span className="text-blue-600 dark:text-blue-400">
                  Побед P2: <strong>{matchStats.wins2}</strong>
                </span>
                <span className="text-gray-600 dark:text-gray-200">
                  Ничьих: <strong>{matchStats.draws}</strong>
                </span>
              </>
            )}
          </div>

          {/* Matches table */}
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-100 dark:bg-gray-800/50">
                <tr>
                  <th className="px-4 py-2 text-left font-medium text-gray-600 dark:text-gray-200">Статус</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-600 dark:text-gray-200">Программа 1</th>
                  <th className="px-4 py-2 text-center font-medium text-gray-600 dark:text-gray-200">Счёт</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-600 dark:text-gray-200">Программа 2</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-600 dark:text-gray-200">Игра</th>
                </tr>
              </thead>
              <tbody>
                {round.matches.map((match) => (
                  <MatchRow key={match.id} match={match} />
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

// Компонент строки матча
function MatchRow({ match }: { match: MatchRound['matches'][0] }) {
  const getStatusBadge = () => {
    switch (match.status) {
      case 'completed':
        return <span className="badge badge-green text-xs">Завершён</span>;
      case 'running':
        return <span className="badge badge-blue text-xs">Выполняется</span>;
      case 'pending':
        return <span className="badge badge-yellow text-xs">В очереди</span>;
      case 'failed':
        return <span className="badge badge-red text-xs">Ошибка</span>;
      default:
        return <span className="badge badge-gray text-xs">{match.status}</span>;
    }
  };

  const getScoreDisplay = () => {
    if (match.status !== 'completed') {
      return <span className="text-gray-400">—</span>;
    }

    const score1Class = match.winner === 1 ? 'text-emerald-600 dark:text-emerald-400 font-bold' : '';
    const score2Class = match.winner === 2 ? 'text-emerald-600 dark:text-emerald-400 font-bold' : '';

    return (
      <span className="font-mono">
        <span className={score1Class}>{match.score1 ?? 0}</span>
        <span className="text-gray-400 mx-1">:</span>
        <span className={score2Class}>{match.score2 ?? 0}</span>
      </span>
    );
  };

  const getProgram1Class = () => {
    if (match.status !== 'completed') return '';
    return match.winner === 1 ? 'font-semibold text-emerald-600 dark:text-emerald-400' : '';
  };

  const getProgram2Class = () => {
    if (match.status !== 'completed') return '';
    return match.winner === 2 ? 'font-semibold text-emerald-600 dark:text-emerald-400' : '';
  };

  return (
    <tr className="border-b border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/30">
      <td className="px-4 py-2">
        {getStatusBadge()}
      </td>
      <td className={`px-4 py-2 ${getProgram1Class()}`}>
        <code className="text-xs bg-gray-100 dark:bg-gray-700 px-1.5 py-0.5 rounded">
          {match.program1_id.slice(0, 8)}
        </code>
      </td>
      <td className="px-4 py-2 text-center">
        {getScoreDisplay()}
      </td>
      <td className={`px-4 py-2 ${getProgram2Class()}`}>
        <code className="text-xs bg-gray-100 dark:bg-gray-700 px-1.5 py-0.5 rounded">
          {match.program2_id.slice(0, 8)}
        </code>
      </td>
      <td className="px-4 py-2">
        <span className="text-gray-600 dark:text-gray-200">{match.game_type}</span>
      </td>
    </tr>
  );
}
