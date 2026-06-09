// Хуки данных поверх TanStack Query.
//
// Заменяют ручной паттерн useState(isLoading/error/data) + useEffect + api.getX():
// кэш, дедупликация параллельных запросов, refetch при фокусе вкладки и
// программная инвалидация по ключам (см. queryKeys) бесплатно.
//
// Поллинг как fallback: компоненты передают refetchInterval только когда
// WebSocket недоступен (см. useTournamentLive) - живое соединение само
// инвалидирует нужные ключи.

import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';
import { queryKeys } from '../api/queryKeys';

/** Интервал fallback-поллинга, когда WS недоступен. */
export const FALLBACK_POLL_INTERVAL = 5000;

type PollOption = { pollInterval?: number | false; enabled?: boolean };

// --- Турниры ---

export function useTournaments(status?: string) {
  return useQuery({
    queryKey: queryKeys.tournaments(status),
    queryFn: () => api.getTournaments(status),
  });
}

export function useTournament(id: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.tournament(id),
    queryFn: () => api.getTournament(id),
    enabled: (opts.enabled ?? true) && !!id,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useLeaderboard(tournamentId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.leaderboard(tournamentId),
    queryFn: () => api.getLeaderboard(tournamentId),
    enabled: (opts.enabled ?? true) && !!tournamentId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useCrossGameLeaderboard(tournamentId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.crossGameLeaderboard(tournamentId),
    queryFn: () => api.getCrossGameLeaderboard(tournamentId),
    enabled: (opts.enabled ?? true) && !!tournamentId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useMatchesByRounds(tournamentId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.matchesByRounds(tournamentId),
    queryFn: () => api.getMatchesByRounds(tournamentId),
    enabled: (opts.enabled ?? true) && !!tournamentId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useTournamentGames(tournamentId: string) {
  return useQuery({
    queryKey: queryKeys.tournamentGames(tournamentId),
    queryFn: () => api.getTournamentGames(tournamentId),
    enabled: !!tournamentId,
  });
}

export function useTournamentGamesStatus(tournamentId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.tournamentGamesStatus(tournamentId),
    queryFn: () => api.getTournamentGamesStatus(tournamentId),
    enabled: (opts.enabled ?? true) && !!tournamentId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useTournamentTeams(tournamentId: string) {
  return useQuery({
    queryKey: queryKeys.tournamentTeams(tournamentId),
    queryFn: () => api.getTournamentTeams(tournamentId),
    enabled: !!tournamentId,
  });
}

export function useMyTeam(tournamentId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.myTeam(tournamentId),
    queryFn: () => api.getMyTeam(tournamentId),
    enabled: (opts.enabled ?? true) && !!tournamentId,
  });
}

// --- Игры ---

export function useGames() {
  return useQuery({
    queryKey: queryKeys.games,
    queryFn: () => api.getGames(),
  });
}

export function useGame(id: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.game(id),
    queryFn: () => api.getGame(id),
    enabled: (opts.enabled ?? true) && !!id,
  });
}

export function useGameLeaderboard(tournamentId: string, gameId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.gameLeaderboard(tournamentId, gameId),
    queryFn: () => api.getGameLeaderboard(tournamentId, gameId),
    enabled: (opts.enabled ?? true) && !!tournamentId && !!gameId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useGamePrograms(tournamentId: string, gameId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.gamePrograms(tournamentId, gameId),
    queryFn: () => api.getGamePrograms(tournamentId, gameId),
    enabled: (opts.enabled ?? true) && !!tournamentId && !!gameId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

// --- Программы ---

export function usePrograms() {
  return useQuery({
    queryKey: queryKeys.programs,
    queryFn: () => api.getPrograms(),
  });
}

export function useProgramVersions(teamId: string, gameId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.programVersions(teamId, gameId),
    queryFn: () => api.getProgramVersions(teamId, gameId),
    enabled: (opts.enabled ?? true) && !!teamId && !!gameId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

// --- Команды ---

export function useTeam(id: string) {
  return useQuery({
    queryKey: queryKeys.team(id),
    queryFn: () => api.getTeam(id),
    enabled: !!id,
  });
}

// --- Admin / system ---

export function useQueueStats(opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.queueStats,
    queryFn: () => api.getQueueStats(),
    enabled: opts.enabled ?? true,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useMatchStatistics(tournamentId?: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.matchStatistics(tournamentId),
    queryFn: () => api.getMatchStatistics(tournamentId),
    enabled: opts.enabled ?? true,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useSystemMetrics(opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.systemMetrics,
    queryFn: () => api.getSystemMetrics(),
    enabled: opts.enabled ?? true,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useSystemHealth(opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.systemHealth,
    queryFn: () => api.getSystemHealth(),
    enabled: opts.enabled ?? true,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useFailedMatches(opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.failedMatches,
    queryFn: () => api.getFailedMatches(),
    enabled: opts.enabled ?? true,
    refetchInterval: opts.pollInterval ?? false,
  });
}
