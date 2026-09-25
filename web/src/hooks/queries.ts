// Хуки данных поверх TanStack Query.
//
// Заменяют ручной паттерн useState(isLoading/error/data) + useEffect + api.getX():
// кэш, дедупликация параллельных запросов, refetch при фокусе вкладки и
// программная инвалидация по ключам (см. queryKeys) бесплатно.
//
// Поллинг как fallback: компоненты передают refetchInterval только когда
// WebSocket недоступен (см. useTournamentLive) - живое соединение само
// инвалидирует нужные ключи.

import { useQuery, keepPreviousData } from '@tanstack/react-query';
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

export function useCrossGameLeaderboard(tournamentId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.crossGameLeaderboard(tournamentId),
    queryFn: ({ signal }) => api.getCrossGameLeaderboard(tournamentId, signal),
    enabled: (opts.enabled ?? true) && !!tournamentId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

export function useMatchesByRounds(tournamentId: string, opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.matchesByRounds(tournamentId),
    queryFn: ({ signal }) => api.getMatchesByRounds(tournamentId, signal),
    enabled: (opts.enabled ?? true) && !!tournamentId,
    refetchInterval: opts.pollInterval ?? false,
  });
}

/** Размер страницы матчей раунда (потолок на бэке - 100). */
export const ROUND_PAGE_SIZE = 50;

// Ключ вложен в matchesByRounds: инвалидация раундов заодно обновляет открытые страницы.
export function useRoundMatches(
  tournamentId: string,
  round: number,
  gameType: string,
  page: number,
  opts: PollOption = {}
) {
  return useQuery({
    queryKey: [...queryKeys.matchesByRounds(tournamentId), round, gameType, page] as const,
    queryFn: ({ signal }) =>
      api.getRoundMatches(tournamentId, round, gameType, ROUND_PAGE_SIZE, page * ROUND_PAGE_SIZE, signal),
    enabled: (opts.enabled ?? true) && !!tournamentId,
    refetchInterval: opts.pollInterval ?? false,
    placeholderData: keepPreviousData,
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
    queryFn: ({ signal }) => api.getTournamentGamesStatus(tournamentId, signal),
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
    queryFn: ({ signal }) => api.getGameLeaderboard(tournamentId, gameId, undefined, signal),
    enabled: (opts.enabled ?? true) && !!tournamentId && !!gameId,
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

export function useFullSystemStatus(opts: PollOption = {}) {
  return useQuery({
    queryKey: queryKeys.fullSystemStatus,
    queryFn: () => api.getFullSystemStatus(),
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
