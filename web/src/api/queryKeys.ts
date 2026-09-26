// Фабрика ключей TanStack Query.
//
// Иерархия ключей позволяет точечно инвалидировать поддеревья:
// invalidateQueries({ queryKey: queryKeys.tournament(id) }) сбросит
// и детали, и лидерборды, и матчи этого турнира.

export const queryKeys = {
  // Tournaments
  tournaments: (status?: string) => ['tournaments', status ?? 'all'] as const,
  tournament: (id: string) => ['tournament', id] as const,
  crossGameLeaderboard: (id: string) => ['tournament', id, 'cross-leaderboard'] as const,
  matchesByRounds: (id: string) => ['tournament', id, 'rounds'] as const,
  tournamentGames: (id: string) => ['tournament', id, 'games'] as const,
  tournamentGamesStatus: (id: string) => ['tournament', id, 'games-status'] as const,
  tournamentTeams: (id: string) => ['tournament', id, 'teams'] as const,
  myTeam: (id: string) => ['tournament', id, 'my-team'] as const,
  gameLeaderboard: (tournamentId: string, gameId: string) =>
    ['tournament', tournamentId, 'game', gameId, 'leaderboard'] as const,
  headToHead: (tournamentId: string, gameId: string) =>
    ['tournament', tournamentId, 'game', gameId, 'head-to-head'] as const,
  ratingHistory: (tournamentId: string, programId: string) =>
    ['tournament', tournamentId, 'program', programId, 'rating-history'] as const,
  gameMatches: (tournamentId: string, gameId: string) =>
    ['tournament', tournamentId, 'game', gameId, 'matches'] as const,

  // Games
  games: ['games'] as const,
  game: (id: string) => ['games', id] as const,

  // Teams
  team: (id: string) => ['team', id] as const,

  // Programs
  programs: ['programs'] as const,
  programVersions: (teamId: string, gameId: string) =>
    ['programs', 'versions', teamId, gameId] as const,

  // Admin / system
  queueStats: ['admin', 'queue-stats'] as const,
  matchStatistics: (tournamentId?: string) => ['admin', 'match-stats', tournamentId ?? 'all'] as const,
  systemMetrics: ['admin', 'system-metrics'] as const,
  fullSystemStatus: ['admin', 'full-system-status'] as const,
  failedMatches: ['admin', 'failed-matches'] as const,
};
