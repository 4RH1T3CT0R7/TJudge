import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import axios from 'axios';
import { useQueries, useQueryClient } from '@tanstack/react-query';
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
import { useAuthStore } from '../store/authStore';
import { SpaceInvader } from '../components/SpaceInvader';
import type { InvaderPose } from '../components/SpaceInvader';
import { CinematicOverlay } from '../components/CinematicOverlay';
import { TerminalLoader } from '../components/TerminalLoader';
import { useDelayedLoading } from '../hooks/useDelayedLoading';
import { useEscapeKey } from '../hooks/useEscapeKey';
import type {
  Tournament,
  TournamentStatus,
  Team,
  Game,
  CrossGameLeaderboardEntry,
  MatchRound,
  TournamentGameWithDetails,
} from '../types';

function extractErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    return err.response?.data?.error || err.response?.data?.message || fallback;
  }
  return err instanceof Error ? err.message : fallback;
}

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
  const tournamentId = id ?? '';
  const { isAuthenticated, user } = useAuthStore();
  const queryClient = useQueryClient();

  // Живые обновления: WS-события точечно инвалидируют кэш (useTournamentLive),
  // а pollInterval включается только как fallback, когда WS недоступен.
  const live = useTournamentLive({ tournamentId, enabled: isAuthenticated });
  const { isConnected } = live;

  const tournamentQuery = useTournament(tournamentId);
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
  useEscapeKey(useCallback(() => setShowJoinModal(false), []), showJoinModal);
  const [isJoining, setIsJoining] = useState(false);
  const [joinError, setJoinError] = useState('');

  // Action states
  const [isStarting, setIsStarting] = useState(false);
  const [isCompleting, setIsCompleting] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  // Games status state (for active game management)
  const [runningGameId, setRunningGameId] = useState<string | null>(null);
  const [settingActiveGameId, setSettingActiveGameId] = useState<string | null>(null);
  const [resettingGameId, setResettingGameId] = useState<string | null>(null);

  // Invader state for WS reactions
  const [wsInvaderPose, setWsInvaderPose] = useState<InvaderPose>('idle');
  const [wsInvaderSpeech, setWsInvaderSpeech] = useState<string | null>(null);
  const wsInvaderTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Cinematic state
  const [cinematicType, setCinematicType] = useState<'tournament_victory' | 'top1_leaderboard' | null>(null);
  const [cinematicTeamName, setCinematicTeamName] = useState('');
  const prevLeaderRankRef = useRef<number | null>(null);

  const flashWsInvader = useCallback((pose: InvaderPose, speech: string | null, duration = 2000) => {
    clearTimeout(wsInvaderTimerRef.current);
    setWsInvaderPose(pose);
    setWsInvaderSpeech(speech);
    wsInvaderTimerRef.current = setTimeout(() => {
      setWsInvaderPose('idle');
      setWsInvaderSpeech(null);
    }, duration);
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
    if (prev && prev !== leaderboardData) {
      flashWsInvader('attack', '// обновление!', 800);
    }

    // Check if user's team reached #1
    if (myTeam && leaderboardData.length > 0) {
      const userEntry = leaderboardData.find(e => e.team_id === myTeam.id);
      if (userEntry) {
        const wasNotFirst = prevLeaderRankRef.current !== null && prevLeaderRankRef.current > 1;
        if (userEntry.rank === 1 && wasNotFirst) {
          setCinematicTeamName(myTeam.name);
          setCinematicType('top1_leaderboard');
        }
        prevLeaderRankRef.current = userEntry.rank;
      }
    }
  }, [leaderboardData, myTeam, flashWsInvader]);

  const tournamentStatusValue = tournament?.status;
  const prevTournamentStatusRef = useRef<TournamentStatus | undefined>(undefined);
  useEffect(() => {
    if (!tournamentStatusValue) return;
    const prev = prevTournamentStatusRef.current;
    prevTournamentStatusRef.current = tournamentStatusValue;
    if (prev && prev !== tournamentStatusValue) {
      flashWsInvader('handsUp', '// турнир обновлён!', 2000);

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

  const matchRoundsData = matchRoundsQuery.data;
  const prevMatchRoundsDataRef = useRef<MatchRound[] | undefined>(undefined);
  useEffect(() => {
    if (!matchRoundsData) return;
    const prev = prevMatchRoundsDataRef.current;
    prevMatchRoundsDataRef.current = matchRoundsData;
    if (prev && prev !== matchRoundsData) {
      flashWsInvader('run', '// матч!', 1000);
    }
  }, [matchRoundsData, flashWsInvader]);

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
      alert(`Перезапущено ${result.enqueued} неудачных матчей`);
      invalidateTournamentData();
    } catch (err: unknown) {
      console.error('Failed to retry matches:', err);
      setActionError(extractErrorMessage(err, 'Не удалось перезапустить матчи'));
    } finally {
      setIsRetryingMatches(false);
    }
  };

  // Helper function to wait for matches to complete and auto-retry if needed
  const waitForMatchesAndAutoRetry = async (targetTournamentId: string, initialEnqueued: number) => {
    const MAX_WAIT_TIME = 10 * 60 * 1000; // 10 minutes max
    const POLL_INTERVAL = 2000; // 2 seconds
    const AUTO_RETRY_THRESHOLD = 50;

    const startTime = Date.now();
    let lastPending = initialEnqueued;

    while (Date.now() - startTime < MAX_WAIT_TIME) {
      await new Promise(resolve => setTimeout(resolve, POLL_INTERVAL));

      try {
        const stats = await api.getMatchStatistics(targetTournamentId);
        const inProgress = stats.pending + stats.running;

        // Refresh leaderboard while matches are running
        if (inProgress !== lastPending) {
          lastPending = inProgress;
          void queryClient.invalidateQueries({ queryKey: queryKeys.crossGameLeaderboard(targetTournamentId) });
          void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(targetTournamentId) });
        }

        // All matches completed
        if (inProgress === 0) {
          // Check for failed matches
          if (stats.failed > 0 && stats.failed <= AUTO_RETRY_THRESHOLD) {
            console.log(`Auto-retrying ${stats.failed} failed matches (threshold: ${AUTO_RETRY_THRESHOLD})`);
            try {
              const retryResult = await api.retryFailedMatches(targetTournamentId);
              if (retryResult.enqueued > 0) {
                // Wait for retry to complete recursively
                await waitForMatchesAndAutoRetry(targetTournamentId, retryResult.enqueued);
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
    if (!tournament || !tournamentId) return;

    setRunningGameId(gameId);
    setActionError(null);
    try {
      const result = await api.runGameMatches(tournamentId, gameName);

      // Find current game index and check if there's a next game
      const currentIndex = games.findIndex(g => g.id === gameId);
      const isLastGame = currentIndex === games.length - 1;

      if (!isLastGame) {
        // Switch to the next game
        const nextGame = games[currentIndex + 1];
        await api.setActiveGame(tournamentId, nextGame.id);
        alert(`Запущено ${result.enqueued} матчей для "${gameDisplayName}". Активная игра переключена на "${nextGame.display_name}". Ожидание завершения матчей...`);
      } else {
        // Last game - deactivate all games
        await api.deactivateAllGames(tournamentId);
        alert(`Запущено ${result.enqueued} матчей для "${gameDisplayName}". Это была последняя игра в турнире. Все игры деактивированы. Ожидание завершения матчей...`);
      }

      // Wait for matches to complete and auto-retry if needed (runs in background)
      void waitForMatchesAndAutoRetry(tournamentId, result.enqueued).then(() => {
        // Final refresh after all matches complete
        void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.crossGameLeaderboard(tournamentId) });
      });

      // Immediate refresh
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
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
    if (!tournamentId) return;

    setSettingActiveGameId(gameId);
    setActionError(null);
    try {
      await api.setActiveGame(tournamentId, gameId);
      // Reload games status
      await queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
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
    if (!tournamentId) return;

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
      const result = await api.resetGameRound(tournamentId, gameId);
      alert(
        `Раунд сброшен успешно!\n\n` +
        `Удалено матчей: ${result.matches_deleted}\n` +
        `Сброшено рейтингов: ${result.participants_reset}\n` +
        `Удалено записей истории: ${result.rating_history_reset}`
      );
      // Reload games status, matches and leaderboard (ratings were reset)
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.crossGameLeaderboard(tournamentId) });
    } catch (err: unknown) {
      console.error('Failed to reset game round:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось сбросить раунд');
    } finally {
      setResettingGameId(null);
    }
  };

  // Ручное обновление матчей (кнопка «Обновить»): refetch стабилен между рендерами.
  const refetchMatches = matchRoundsQuery.refetch;
  const refreshMatches = useCallback(() => {
    void refetchMatches();
  }, [refetchMatches]);
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
      {/* Cinematic overlay for tournament victory / #1 */}
      {cinematicType && (
        <CinematicOverlay
          type={cinematicType}
          teamName={cinematicTeamName}
          onComplete={() => setCinematicType(null)}
        />
      )}

      {/* Header */}
      <div className="mb-8">
        <Link to="/tournaments" className="inline-flex items-center gap-2 text-gray-400 hover:text-primary-400 mb-4 transition-colors">
          <ArrowLeftIcon />
          <span>Назад к турнирам</span>
        </Link>

        <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
          <div>
            <div className="flex flex-wrap items-center gap-3 mb-3">
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
              <HashtagIcon />
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
      <div className="bg-gray-900 rounded-lg border border-gray-800 mb-6 p-1.5">
        <nav className="flex gap-1 overflow-x-auto items-center">
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
                      : 'bg-gray-700 text-gray-300'
                  }`}>
                    {tab.count}
                  </span>
                )}
              </button>
            );
          })}
          {/* WS-reactive mini invader */}
          {isConnected && wsInvaderPose !== 'idle' && (
            <div className="ml-auto px-2">
              <SpaceInvader size="sm" controlledPose={wsInvaderPose} speechBubble={wsInvaderSpeech} />
            </div>
          )}
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
            isAdmin={isAdmin}
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
              if (!window.confirm('Дисквалифицировать команду? Все матчи с её участием будут удалены.')) return;
              try {
                await api.disqualifyTeam(teamId);
                invalidateTournamentData();
              } catch (err) {
                console.error('Failed to disqualify team:', err);
                window.alert('Не удалось дисквалифицировать команду. Попробуйте снова.');
              }
            }}
            onRestore={async (teamId) => {
              if (!window.confirm('Восстановить команду? Она снова сможет участвовать в турнире.')) return;
              try {
                await api.restoreTeam(teamId);
                invalidateTournamentData();
              } catch (err) {
                console.error('Failed to restore team:', err);
                window.alert('Не удалось восстановить команду. Попробуйте снова.');
              }
            }}
          />
        )}
      </div>

      {/* Join Modal */}
      {showJoinModal && (
        <div className="modal-backdrop" onClick={() => setShowJoinModal(false)}>
          <div className="modal-content w-full max-w-md p-6 m-4" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-xl font-bold text-gray-100">Участие в турнире</h2>
              <button
                onClick={() => setShowJoinModal(false)}
                aria-label="Закрыть"
                className="p-2 hover:bg-gray-800 rounded-lg transition-colors"
              >
                <XMarkIcon />
              </button>
            </div>

            <div className="space-y-6">
              <div>
                <h3 className="font-semibold text-gray-100 mb-3">Создать новую команду</h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    name="teamName"
                    autoComplete="off"
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
                  <div className="w-full border-t border-gray-700" />
                </div>
                <div className="relative flex justify-center text-sm">
                  <span className="px-4 bg-gray-900 text-gray-400">или</span>
                </div>
              </div>

              <div>
                <h3 className="font-semibold text-gray-100 mb-3">Присоединиться к существующей</h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    name="joinCode"
                    autoComplete="off"
                    value={joinCode}
                    onChange={(e) => { setJoinCode(e.target.value); setJoinError(''); }}
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
                {joinError && <p className="text-red-400 text-sm mt-1">{joinError}</p>}
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
          <p className="text-gray-300 leading-relaxed">{tournament.description}</p>
        </div>
      ) : (
        <p className="text-gray-400 mb-8">Описание не указано.</p>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="stat-card">
          <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
            <UsersIcon />
            <span>Макс. размер команды</span>
          </div>
          <p className="text-2xl font-bold text-gray-100">{tournament.max_team_size}</p>
        </div>

        {tournament.max_participants && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <UsersIcon />
              <span>Макс. участников</span>
            </div>
            <p className="text-2xl font-bold text-gray-100">{tournament.max_participants}</p>
          </div>
        )}

        {tournament.start_time && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <CalendarIcon />
              <span>Начало</span>
            </div>
            <p className="text-lg font-bold text-gray-100">
              {new Date(tournament.start_time).toLocaleDateString('ru-RU')}
            </p>
          </div>
        )}

        {tournament.end_time && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <CalendarIcon />
              <span>Окончание</span>
            </div>
            <p className="text-lg font-bold text-gray-100">
              {new Date(tournament.end_time).toLocaleDateString('ru-RU')}
            </p>
          </div>
        )}

        <div className="stat-card">
          <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
            <ClockIcon />
            <span>Создан</span>
          </div>
          <p className="text-lg font-bold text-gray-100">
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
    <div className="mb-8 p-6 bg-gradient-to-b from-primary-900/30 via-primary-800/20 to-transparent rounded-2xl">
      <div className="text-center mb-6">
        <h3 className="text-2xl font-bold text-gray-100 mb-1">
          🏆 Победители турнира 🏆
        </h3>
        <p className="text-gray-400">Поздравляем финалистов!</p>
      </div>

      <div className="flex items-end justify-center gap-4 max-w-2xl mx-auto">
        {podiumData.map(({ entry, place, height, delay, bgGradient, textColor, medal }) => (
          <div
            key={place}
            className={`flex-1 max-w-48 transition-[transform,opacity] duration-700 ease-out ${
              isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-10'
            } ${delay}`}
          >
            {/* Winner card */}
            <div className={`text-center mb-2 transform transition-[transform,opacity] duration-500 ${
              isVisible ? 'scale-100' : 'scale-0'
            } ${delay}`}>
              <div className="text-4xl mb-2 animate-bounce" style={{ animationDelay: `${(place - 1) * 200}ms`, animationDuration: '2s' }}>
                {medal}
              </div>
              <div className="font-bold text-lg text-gray-100 truncate px-2">
                {entry.team_name || entry.program_name}
              </div>
              <div className="text-2xl font-bold bg-gradient-to-r from-primary-600 to-primary-400 bg-clip-text text-transparent">
                {entry.total_rating.toLocaleString()}
              </div>
              <div className="text-xs text-gray-400">
                {entry.total_wins}W / {entry.total_losses}L
              </div>
            </div>

            {/* Podium */}
            <div
              className={`${height} bg-gradient-to-t ${bgGradient} rounded-t-lg shadow-lg relative overflow-hidden transition-[transform,opacity] duration-700 ease-out ${
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
              animationDuration: `${2 + (i * 0.37) % 1}s`,
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
  // Поллинг каждые 2с удалён: живые данные приходят через WS-инвалидации
  // (useTournamentLive) либо fallback-поллинг TanStack Query на уровне страницы.
  // Флаг оставлен для индикатора «Обновление...» в шапке вкладки.
  const [autoRefresh, setAutoRefresh] = useState(true);

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-bold text-gray-100">Рейтинг</h2>
          {isConnected && (
            <span className="online-indicator">
              Онлайн
            </span>
          )}
          {hasActiveMatches && autoRefresh && (
            <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded-full bg-blue-900/30 text-blue-400 text-xs">
              <span className="w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
              Обновление...
            </span>
          )}
          {isRefreshing && (
            <div className="w-4 h-4 border-2 border-primary-800 border-t-primary-400 rounded-full animate-spin" />
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
        isDark ? 'bg-gray-700 text-gray-300' : 'bg-gray-800 text-gray-300'
      }`}>
        {rank}
      </div>
    );
  };

  const getRowClass = (index: number) => {
    if (index === 0) return isDark ? 'bg-amber-900/10' : 'bg-amber-900/10';
    if (index === 1) return isDark ? 'bg-gray-700/20' : 'bg-gray-700/20';
    if (index === 2) return isDark ? 'bg-orange-900/10' : 'bg-orange-900/10';
    return '';
  };

  return (
    <div className={isDark
      ? 'grid grid-cols-2 xl:grid-cols-3 gap-2'
      : 'space-y-2'
    }>
      {/* Card-style entries */}
      {entries.map((entry, index) => (
        <div
          key={entry.program_id}
          className={`${isDark ? 'p-2.5' : 'p-4'} rounded-xl transition-colors ${
            isDark
              ? `bg-gray-800/50 border border-gray-700 ${getRowClass(index)}`
              : `bg-gray-800/50 border border-gray-800 ${getRowClass(index)} hover:shadow-md`
          }`}
        >
          <div className={`flex items-center ${isDark ? 'gap-3' : 'gap-4'}`}>
            {/* Rank */}
            {isDark ? (
              <div className={`w-7 h-7 rounded-full flex items-center justify-center text-sm font-bold shrink-0 ${
                index === 0 ? 'bg-gradient-to-br from-yellow-300 to-amber-500 text-amber-900' :
                index === 1 ? 'bg-gradient-to-br from-gray-200 to-gray-400 text-gray-700' :
                index === 2 ? 'bg-gradient-to-br from-orange-300 to-orange-500 text-orange-900' :
                'bg-gray-700 text-gray-300'
              }`}>
                {entry.rank}
              </div>
            ) : getRankBadge(entry.rank)}

            {/* Team Info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <h3 className={`font-bold truncate ${isDark ? 'text-sm text-white' : 'text-lg text-gray-100'}`}>
                    {entry.team_name || entry.program_name}
                  </h3>
                  {!isDark && (
                    <div className="flex items-center gap-3 text-sm text-gray-400">
                      <span>{entry.total_games} игр</span>
                      <span>•</span>
                      <span className="text-emerald-400">{entry.total_wins}W</span>
                      <span className="text-red-400">{entry.total_losses}L</span>
                    </div>
                  )}
                </div>

                {/* Score */}
                <div className="text-right shrink-0">
                  <div className={`font-bold tabular-nums ${isDark ? 'text-xl' : 'text-3xl'} ${
                    index === 0 ? 'text-amber-500' :
                    index === 1 ? 'text-gray-400' :
                    index === 2 ? 'text-orange-500' :
                    'text-primary-400'
                  }`}>
                    {entry.total_rating.toLocaleString()}
                  </div>
                  {!isDark && (
                    <div className="text-xs text-gray-400">
                      очков
                    </div>
                  )}
                </div>
              </div>

              {/* Score bar - только в обычном режиме */}
              {!isDark && (
                <div className="mt-3 h-2 bg-gray-700 rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-[width] duration-500 ${
                      index === 0 ? 'bg-gradient-to-r from-amber-400 to-amber-500' :
                      index === 1 ? 'bg-gradient-to-r from-gray-500 to-gray-600' :
                      index === 2 ? 'bg-gradient-to-r from-orange-400 to-orange-500' :
                      'bg-gradient-to-r from-primary-400 to-primary-500'
                    }`}
                    style={{ width: `${(entry.total_rating / maxScore) * 100}%` }}
                  />
                </div>
              )}
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
  isCompact = false,
}: {
  entries: CrossGameLeaderboardEntry[];
  games: Game[];
  isDark?: boolean;
  isCompact?: boolean;
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

  const cellPx = isCompact ? 'px-3 py-1' : 'px-4 py-3';
  const headPx = isCompact ? 'px-3 py-1.5' : 'px-4 py-3';
  const headText = isCompact ? 'text-[10px]' : 'text-sm';

  return (
    <div className={`overflow-x-auto ${isDark ? '' : 'card p-0'}`}>
      <table className={`w-full ${isDark ? 'text-white' : 'text-gray-100'} ${isCompact ? 'text-sm' : ''}`}>
        <thead className={isDark ? 'bg-gray-800/50' : 'bg-gray-800/50'}>
          <tr>
            <th className={`${headPx} text-left font-semibold ${headText} uppercase tracking-wide`}>Место</th>
            <th className={`${headPx} text-left font-semibold ${headText} uppercase tracking-wide`}>Команда</th>
            {games.map((game) => (
              <th key={game.id} className={`${headPx} text-center font-semibold ${headText} uppercase tracking-wide`}>
                {game.display_name}
              </th>
            ))}
            <th className={`${headPx} text-right font-semibold ${headText} uppercase tracking-wide`}>Сумма</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry, index) => (
            <tr
              key={entry.program_id}
              className={`border-b ${isDark ? 'border-gray-700/50' : 'border-gray-700'} ${getRowClass(index)} transition-colors`}
            >
              <td className={cellPx}>
                <span className={getRankClass(index)}>
                  {entry.rank}
                </span>
              </td>
              <td className={cellPx}>
                <span className="font-semibold">
                  {entry.team_name || entry.program_name}
                </span>
              </td>
              {games.map((game) => {
                const gameRating = entry.game_ratings[game.id];
                return (
                  <td key={game.id} className={`${cellPx} text-center`}>
                    {gameRating ? (
                      <div>
                        <span className="font-mono font-bold">{Math.round(gameRating.rating)}</span>
                        {!isCompact && (
                          <div className={`text-xs ${isDark ? 'text-gray-400' : 'text-gray-400'}`}>
                            <span className="text-emerald-500" title="Побед">{gameRating.wins}</span>
                            <span className="mx-0.5">/</span>
                            <span className="text-red-500" title="Поражений">{gameRating.losses}</span>
                            <span className="mx-0.5">/</span>
                            <span title="Ничьих">{gameRating.draws || 0}</span>
                          </div>
                        )}
                      </div>
                    ) : (
                      <span className="text-gray-400">-</span>
                    )}
                  </td>
                );
              })}
              <td className={`${cellPx} text-right`}>
                <span className={`font-mono font-bold ${isCompact ? 'text-base' : 'text-lg'} ${isDark ? 'text-primary-400' : 'text-primary-400'}`}>
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
  return <CrossGameLeaderboardTable entries={entries} games={games} isDark isCompact />;
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
  matchRounds,
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
  matchRounds?: MatchRound[];
}) {
  const queryClient = useQueryClient();

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

  const handleToggleAutoRound = async (e: React.MouseEvent, gameId: string, currentStatus: TournamentGameWithDetails | undefined) => {
    e.preventDefault();
    e.stopPropagation();
    if (!tournamentId) return;

    const isEnabled = currentStatus?.auto_round_enabled ?? false;

    if (!isEnabled) {
      const intervalStr = window.prompt('Интервал авто-раунда (секунды, 10-3600):', '60');
      if (!intervalStr) return;
      const interval = parseInt(intervalStr, 10);
      if (isNaN(interval) || interval < 10 || interval > 3600) {
        alert('Интервал должен быть от 10 до 3600 секунд');
        return;
      }
      try {
        await api.setAutoRound(tournamentId, gameId, true, interval);
        await queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      } catch (err) {
        console.error('Failed to enable auto-round:', err);
        alert('Не удалось включить авто-раунд');
      }
    } else {
      try {
        await api.setAutoRound(tournamentId, gameId, false, currentStatus?.auto_round_interval_seconds ?? 60);
        await queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      } catch (err) {
        console.error('Failed to disable auto-round:', err);
        alert('Не удалось выключить авто-раунд');
      }
    }
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
        <div className="mb-6 p-4 bg-blue-900/20 border border-blue-700 rounded-lg">
          <div className="flex items-start gap-3">
            <div className="shrink-0 w-8 h-8 bg-blue-800 rounded-lg flex items-center justify-center">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-blue-400">
                <path strokeLinecap="round" strokeLinejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />
              </svg>
            </div>
            <div>
              <h4 className="font-medium text-blue-200">Режим администратора</h4>
              <p className="text-sm text-blue-300 mt-1">
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
          const hasActiveMatches = (matchRounds || []).some(
            r => r.game_type === game.name && (r.pending_count > 0 || r.running_count > 0)
          );
          const roundInProgress = currentRound > 0 && gameStatus?.round_completed === false;
          const isRoundRunning = hasActiveMatches || roundInProgress;

          return (
            <Link
              key={game.id}
              to={`/tournaments/${tournamentId}/games/${game.id}`}
              className={`card card-interactive group relative overflow-hidden ${
                isActive ? 'ring-2 ring-green-600' : ''
              }`}
            >
              {/* Game number badge */}
              <div className="absolute top-3 right-3 w-8 h-8 bg-gray-800 rounded-full flex items-center justify-center text-sm font-bold text-gray-300">
                {index + 1}
              </div>

              <div className="mb-3 pr-10">
                <h3 className="text-lg font-bold text-gray-100 group-hover:text-primary-400 transition-colors">
                  {game.display_name}
                </h3>
                <div className="flex items-center gap-2 mt-1 flex-wrap">
                  <code className="text-sm bg-gray-800 px-2 py-0.5 rounded text-gray-400">
                    {game.name}
                  </code>
                  {currentRound > 0 && (
                    <span className="text-xs text-gray-400">
                      • Раунд {currentRound}
                    </span>
                  )}
                  {isActive && (
                    <span className="px-2 py-0.5 bg-green-900/50 text-green-400 text-xs rounded-full font-medium">
                      Активна
                    </span>
                  )}
                </div>
              </div>

              {game.rules && (
                <p className="text-gray-300 text-sm line-clamp-3 mb-4">
                  {game.rules.substring(0, 200)}...
                </p>
              )}

              <div className="flex items-center justify-between pt-3 border-t border-gray-700">
                {myTeam && !isAdmin && (
                  <div className="flex items-center gap-2 text-primary-400 text-sm font-medium">
                    <PlayIcon />
                    <span>Управление программой</span>
                  </div>
                )}

                {/* Admin controls */}
                {isAdmin && tournamentStatus === 'active' && (
                  <div className="flex flex-col gap-2 w-full">
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
                      <div className="flex gap-2">
                        <button
                          onClick={(e) => handleRunMatches(e, game)}
                          disabled={runningGameId === game.id || isRoundRunning}
                          className="btn btn-primary text-xs py-1.5 px-3 flex-1"
                        >
                          {runningGameId === game.id ? (
                            <>
                              <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                              Запуск...
                            </>
                          ) : isRoundRunning ? (
                            <>
                              <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                              Выполняется...
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
                        <button
                          onClick={(e) => handleToggleAutoRound(e, game.id, gameStatus)}
                          className={`btn text-xs py-1.5 px-3 ${
                            gameStatus?.auto_round_enabled
                              ? 'bg-green-600 hover:bg-green-700 text-white'
                              : 'bg-gray-600 hover:bg-gray-700 text-gray-200'
                          }`}
                          title={gameStatus?.auto_round_enabled
                            ? `Авто-раунд: каждые ${gameStatus.auto_round_interval_seconds}с`
                            : 'Включить авто-раунд'
                          }
                        >
                          {gameStatus?.auto_round_enabled
                            ? `Авто ✓ (${gameStatus.auto_round_interval_seconds}с)`
                            : 'Авто'}
                        </button>
                      </div>
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
  isAdmin,
  myTeam,
  tournamentStatus,
  onJoinByCode,
  joinCode,
  setJoinCode,
  isJoining,
  joinError,
  setJoinError,
  onDisqualify,
  onRestore,
}: {
  teams: Team[];
  isAuthenticated: boolean;
  isAdmin: boolean;
  myTeam: Team | null;
  tournamentStatus: TournamentStatus;
  onJoinByCode: () => void;
  joinCode: string;
  setJoinCode: (code: string) => void;
  isJoining: boolean;
  joinError: string;
  setJoinError: (e: string) => void;
  onDisqualify?: (teamId: string) => void;
  onRestore?: (teamId: string) => void;
}) {
  const showJoinSection = isAuthenticated && !myTeam && tournamentStatus === 'pending';
  const [membersExpanded, setMembersExpanded] = useState(false);

  // Составы команд: запросы выполняются только после раскрытия (enabled),
  // кэшируются по queryKeys.team и дедуплицируются между перерисовками.
  const memberQueries = useQueries({
    queries: teams.map((t) => ({
      queryKey: queryKeys.team(t.id),
      queryFn: () => api.getTeam(t.id),
      enabled: membersExpanded,
    })),
  });
  const loadingMembers = membersExpanded && memberQueries.some(q => q.isLoading);
  const teamMembers: Record<string, { username: string; email: string }[]> = {};
  teams.forEach((t, i) => {
    const data = memberQueries[i]?.data;
    if (data) {
      teamMembers[t.id] = data.members.map(m => ({ username: m.username, email: m.email }));
    }
  });

  const toggleAllMembers = () => {
    setMembersExpanded(prev => !prev);
  };

  return (
    <div>
      {/* Join by code section */}
      {showJoinSection && (
        <div className="card mb-6 bg-blue-900/30 border-blue-700">
          <h3 className="font-semibold mb-3 text-blue-200">Присоединиться к команде</h3>
          <p className="text-sm text-blue-300 mb-3">
            Введите код приглашения, полученный от капитана команды
          </p>
          <div className="flex gap-2">
            <input
              type="text"
              value={joinCode}
              onChange={(e) => { setJoinCode(e.target.value.toUpperCase()); setJoinError(''); }}
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
          {joinError && <p className="text-red-400 text-sm mt-2">{joinError}</p>}
        </div>
      )}

      {/* Teams list */}
      {teams.length === 0 ? (
        <div className="empty-state">
          <div className="flex justify-center mb-4">
            <SpaceInvader size="sm" controlledPose="cry" speechBubble="// пока никого..." eyeOverride="sad" />
          </div>
          <h3 className="empty-state-title">Нет команд</h3>
          <p className="empty-state-description">
            Ни одна команда еще не присоединилась к турниру
          </p>
        </div>
      ) : (
        <div>
          {isAdmin && (
            <button
              onClick={toggleAllMembers}
              disabled={loadingMembers}
              className="mb-4 inline-flex items-center gap-1.5 text-sm text-gray-400 hover:text-gray-200 transition-colors"
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className={`w-4 h-4 transition-transform ${membersExpanded ? 'rotate-180' : ''}`}>
                <path strokeLinecap="round" strokeLinejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
              </svg>
              {loadingMembers ? 'Загрузка...' : membersExpanded ? 'Скрыть составы' : 'Показать составы команд'}
            </button>
          )}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {teams.map((team, index) => (
              <div
                key={team.id}
                className={`card group hover:shadow-lg hover:shadow-gray-900/50 transition-shadow ${
                  myTeam?.id === team.id
                    ? 'border-2 border-primary-500 bg-primary-900/20'
                    : team.is_disqualified
                    ? 'border border-red-800/50 opacity-60'
                    : ''
                }`}
              >
                <div className="flex items-center gap-3">
                  <div className={`w-10 h-10 rounded-lg flex items-center justify-center text-white font-bold ${
                    team.is_disqualified ? 'bg-red-700' : myTeam?.id === team.id ? 'bg-primary-500' : 'bg-gray-500'
                  }`}>
                    {index + 1}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold text-gray-100 truncate">{team.name}</h3>
                      {team.is_disqualified && (
                        <span className="px-2 py-0.5 bg-red-900/50 text-red-400 text-xs rounded-full">Дисквалификация</span>
                      )}
                      {myTeam?.id === team.id && (
                        <span className="badge badge-blue text-xs">Ваша</span>
                      )}
                    </div>
                    <p className="text-sm text-gray-400">
                      {new Date(team.created_at).toLocaleDateString('ru-RU')}
                    </p>
                  </div>
                </div>

                {/* Admin: team members */}
                {isAdmin && membersExpanded && (
                  <div className="mt-3 pt-3 border-t border-gray-700">
                    {loadingMembers ? (
                      <p className="text-xs text-gray-500">Загрузка...</p>
                    ) : (teamMembers[team.id] || []).length === 0 ? (
                      <p className="text-xs text-gray-500">Нет участников</p>
                    ) : (
                      <div className="space-y-1">
                        {teamMembers[team.id].map((member, i) => (
                          <div key={i} className="flex items-center gap-2 text-sm">
                            <span className="w-5 h-5 rounded-full bg-gray-600 flex items-center justify-center text-xs text-gray-300">
                              {member.username[0]?.toUpperCase()}
                            </span>
                            <span className="text-gray-200">{member.username}</span>
                            <span className="text-gray-500 text-xs truncate">{member.email}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {isAdmin && tournamentStatus === 'active' && (
                  <div className="mt-3 pt-3 border-t border-gray-700">
                    {team.is_disqualified ? (
                      <button
                        onClick={() => onRestore?.(team.id)}
                        className="px-3 py-1.5 bg-green-700 hover:bg-green-600 text-white text-xs rounded-lg transition-colors"
                      >
                        Восстановить
                      </button>
                    ) : (
                      <button
                        onClick={() => onDisqualify?.(team.id)}
                        className="px-3 py-1.5 bg-red-700 hover:bg-red-600 text-white text-xs rounded-lg transition-colors"
                      >
                        Дисквалифицировать
                      </button>
                    )}
                  </div>
                )}

                {myTeam?.id === team.id && (
                  <Link
                    to={`/teams/${team.id}`}
                    className="mt-3 inline-flex items-center gap-1 text-primary-400 hover:text-primary-300 text-sm font-medium"
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
        </div>
      )}
    </div>
  );
}

// Matches Tab Component - отображает матчи, сгруппированные по раундам
function MatchesTab({
  rounds,
  onRefresh,
  isRefreshing,
  isAdmin
}: {
  rounds: MatchRound[];
  onRefresh: () => void;
  isRefreshing: boolean;
  isAdmin: boolean;
}) {
  const [expandedRounds, setExpandedRounds] = useState<Set<string>>(new Set());
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [hiddenRounds, setHiddenRounds] = useState<Set<string>>(new Set());

  const hideRound = (roundKey: string) => {
    setHiddenRounds(prev => {
      const next = new Set(prev);
      next.add(roundKey);
      return next;
    });
  };

  const showAllRounds = () => setHiddenRounds(new Set());

  // Проверяем, есть ли активные матчи (pending или running)
  // Поллинг каждые 2с удалён: живые данные приходят через WS-инвалидации
  // (useTournamentLive) либо fallback-поллинг TanStack Query на уровне страницы.
  const hasActiveMatches = rounds.some(
    r => r.pending_count > 0 || r.running_count > 0
  );

  const toggleRound = (roundKey: string) => {
    setExpandedRounds(prev => {
      const next = new Set(prev);
      if (next.has(roundKey)) {
        next.delete(roundKey);
      } else {
        next.add(roundKey);
      }
      return next;
    });
  };

  const expandAll = () => {
    setExpandedRounds(new Set(rounds.map(r => `${r.round_number}-${r.game_type}`)));
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
            <h2 className="text-xl font-bold text-gray-100">
              Матчи по раундам
            </h2>
            {hasActiveMatches && autoRefresh && (
              <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded-full bg-blue-900/30 text-blue-400 text-xs">
                <span className="w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
                Обновление...
              </span>
            )}
            {isRefreshing && (
              <div className="w-4 h-4 border-2 border-primary-800 border-t-primary-400 rounded-full animate-spin" />
            )}
          </div>
          <div className="flex flex-wrap gap-3 text-sm">
            <span className="text-gray-300">
              Всего: <strong className="text-gray-100">{totalStats.total}</strong>
            </span>
            <span className="text-emerald-400">
              Завершено: <strong>{totalStats.completed}</strong>
            </span>
            {totalStats.running > 0 && (
              <span className="text-blue-400">
                Выполняется: <strong>{totalStats.running}</strong>
              </span>
            )}
            {totalStats.pending > 0 && (
              <span className="text-yellow-400">
                В очереди: <strong>{totalStats.pending}</strong>
              </span>
            )}
            {totalStats.failed > 0 && (
              <span className="text-red-400">
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
            <span className="text-sm font-medium text-gray-300">
              Общий прогресс
            </span>
            <span className="text-sm font-mono text-gray-300">
              {totalStats.completed} / {totalStats.total} ({Math.round((totalStats.completed / totalStats.total) * 100)}%)
            </span>
          </div>
          <div className="w-full h-4 bg-gray-700 rounded-full overflow-hidden">
            <div className="h-full flex">
              {/* Completed - green */}
              <div
                className="bg-emerald-500 transition-[width] duration-500"
                style={{ width: `${(totalStats.completed / totalStats.total) * 100}%` }}
              />
              {/* Running - blue, animated */}
              <div
                className="bg-blue-500 animate-pulse transition-[width] duration-500"
                style={{ width: `${(totalStats.running / totalStats.total) * 100}%` }}
              />
              {/* Failed - red */}
              <div
                className="bg-red-500 transition-[width] duration-500"
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
                <span className="w-3 h-3 rounded-full bg-gray-600" />
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

      {/* Hidden rounds notice */}
      {hiddenRounds.size > 0 && (
        <div className="mb-4 flex items-center gap-3 text-sm text-gray-400">
          <span>Скрыто раундов: {hiddenRounds.size}</span>
          <button onClick={showAllRounds} className="text-primary-400 hover:text-primary-300 underline">
            Показать все
          </button>
        </div>
      )}

      {/* Rounds list */}
      <div className="space-y-3">
        {rounds
          .filter(r => !hiddenRounds.has(`${r.round_number}-${r.game_type}`))
          .map((round) => {
            const roundKey = `${round.round_number}-${round.game_type}`;
            return (
              <RoundCard
                key={roundKey}
                round={round}
                isExpanded={expandedRounds.has(roundKey)}
                onToggle={() => toggleRound(roundKey)}
                isAdmin={isAdmin}
                onHide={() => hideRound(roundKey)}
              />
            );
          })}
      </div>
    </div>
  );
}

// Game name display mapping
const gameDisplayNames: Record<string, string> = {
  dilemma: 'Дилемма заключённого',
  tug_of_war: 'Перетягивание каната',
  travelers_dilemma: 'Дилемма путешественника',
  public_goods: 'Общественное благо',
  dollar_auction: 'Аукцион двойной цены',
};

const getGameDisplayName = (gameType: string) => gameDisplayNames[gameType] || gameType;

// Компонент карточки раунда
function RoundCard({
  round,
  isExpanded,
  onToggle,
  isAdmin,
  onHide,
}: {
  round: MatchRound;
  isExpanded: boolean;
  onToggle: () => void;
  isAdmin: boolean;
  onHide: () => void;
}) {
  const getStatusColor = () => {
    if (round.failed_count > 0) return 'border-l-red-500';
    if (round.running_count > 0) return 'border-l-blue-500';
    if (round.pending_count > 0) return 'border-l-yellow-500';
    if (round.completed_count === round.total_matches) return 'border-l-emerald-500';
    return 'border-l-gray-600';
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
        className="w-full px-4 py-3 flex items-center justify-between hover:bg-gray-800/50 transition-colors"
      >
        <div className="flex items-center gap-3">
          <div className="text-gray-400">
            {isExpanded ? <ChevronDownIcon /> : <ChevronRightIcon />}
          </div>
          <div className="flex items-center gap-2">
            <FolderIcon />
            <span className="font-semibold text-gray-100">
              Раунд {round.round_number}
            </span>
            <span className="px-2 py-0.5 bg-primary-900/30 text-primary-400 text-xs rounded-full font-medium">
              {getGameDisplayName(round.game_type)}
            </span>
          </div>
          <span className="text-sm text-gray-400">
            {round.total_matches} матчей
          </span>
        </div>

        <div className="flex items-center gap-4">
          {/* Mini stats badges */}
          <div className="hidden sm:flex items-center gap-2 text-xs">
            {round.completed_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-emerald-900/30 text-emerald-400">
                {round.completed_count} завершено
              </span>
            )}
            {round.running_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-blue-900/30 text-blue-400">
                {round.running_count} выполняется
              </span>
            )}
            {round.pending_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-yellow-900/30 text-yellow-400">
                {round.pending_count} в очереди
              </span>
            )}
            {round.failed_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-red-900/30 text-red-400">
                {round.failed_count} ошибок
              </span>
            )}
          </div>

          {/* Progress bar */}
          <div className="w-24 h-2 bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-emerald-500 transition-[width] duration-300"
              style={{ width: `${getProgressPercent()}%` }}
            />
          </div>
          <span className="text-sm font-mono text-gray-300 w-12 text-right">
            {getProgressPercent()}%
          </span>
          {isAdmin && round.failed_count > 0 && (
            <button
              onClick={(e) => { e.stopPropagation(); onHide(); }}
              className="ml-2 px-2 py-1 text-xs text-red-400 hover:text-red-300 hover:bg-red-900/30 rounded transition-colors"
              title="Скрыть этот раунд"
            >
              ✕
            </button>
          )}
        </div>
      </button>

      {/* Expanded content */}
      {isExpanded && (
        <div className="border-t border-gray-800">
          {/* Round summary */}
          <div className="px-4 py-3 bg-gray-800/30 flex flex-wrap gap-4 text-sm">
            <span className="text-gray-300">
              Дата: <strong className="text-gray-100">
                {new Date(round.created_at).toLocaleString('ru-RU')}
              </strong>
            </span>
            {round.completed_count > 0 && (
              <>
                <span className="text-emerald-400">
                  Побед P1: <strong>{matchStats.wins1}</strong>
                </span>
                <span className="text-blue-400">
                  Побед P2: <strong>{matchStats.wins2}</strong>
                </span>
                <span className="text-gray-300">
                  Ничьих: <strong>{matchStats.draws}</strong>
                </span>
              </>
            )}
          </div>

          {/* Matches table */}
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-800/50">
                <tr>
                  <th className="px-4 py-2 text-left font-medium text-gray-300">Статус</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-300">Программа 1</th>
                  <th className="px-4 py-2 text-center font-medium text-gray-300">Счёт</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-300">Программа 2</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-300">Игра</th>
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

    const score1Class = match.winner === 1 ? 'text-emerald-400 font-bold' : '';
    const score2Class = match.winner === 2 ? 'text-emerald-400 font-bold' : '';

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
    return match.winner === 1 ? 'font-semibold text-emerald-400' : '';
  };

  const getProgram2Class = () => {
    if (match.status !== 'completed') return '';
    return match.winner === 2 ? 'font-semibold text-emerald-400' : '';
  };

  return (
    <tr className="border-b border-gray-700 hover:bg-gray-800/30">
      <td className="px-4 py-2">
        {getStatusBadge()}
      </td>
      <td className={`px-4 py-2 ${getProgram1Class()}`}>
        <code className="text-xs bg-gray-800 px-1.5 py-0.5 rounded">
          {match.program1_id.slice(0, 8)}
        </code>
      </td>
      <td className="px-4 py-2 text-center">
        {getScoreDisplay()}
      </td>
      <td className={`px-4 py-2 ${getProgram2Class()}`}>
        <code className="text-xs bg-gray-800 px-1.5 py-0.5 rounded">
          {match.program2_id.slice(0, 8)}
        </code>
      </td>
      <td className="px-4 py-2">
        <span className="text-gray-300">{match.game_type}</span>
      </td>
    </tr>
  );
}
