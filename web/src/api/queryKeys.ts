// Фабрика ключей TanStack Query.
//
// Иерархия ключей позволяет точечно инвалидировать поддеревья:
// invalidateQueries({ queryKey: queryKeys.tournament(id) }) сбросит
// и детали, и лидерборды, и матчи этого турнира.

export const queryKeys = {
  // Auth
  me: ['me'] as const,

  // Tournaments
  tournaments: (status?: string) => ['tournaments', status ?? 'all'] as const,
  tournament: (id: string) => ['tournament', id] as const,
  leaderboard: (id: string) => ['tournament', id, 'leaderboard'] as const,
  crossGameLeaderboard: (id: string) => ['tournament', id, 'cross-leaderboard'] as const,
  tournamentMatches: (id: string) => ['tournament', id, 'matches'] as const,
  matchesByRounds: (id: string) => ['tournament', id, 'rounds'] as const,
  tournamentGames: (id: string) => ['tournament', id, 'games'] as const,
  tournamentGamesStatus: (id: string) => ['tournament', id, 'games-status'] as const,
  activeGame: (id: string) => ['tournament', id, 'active-game'] as const,
  tournamentTeams: (id: string) => ['tournament', id, 'teams'] as const,
  myTeam: (id: string) => ['tournament', id, 'my-team'] as const,
  gameLeaderboard: (tournamentId: string, gameId: string) =>
    ['tournament', tournamentId, 'game', gameId, 'leaderboard'] as const,
  gameMatches: (tournamentId: string, gameId: string) =>
    ['tournament', tournamentId, 'game', gameId, 'matches'] as const,
  gamePrograms: (tournamentId: string, gameId: string) =>
    ['tournament', tournamentId, 'game', gameId, 'programs'] as const,
  autoRound: (tournamentId: string, gameId: string) =>
    ['tournament', tournamentId, 'game', gameId, 'auto-round'] as const,

  // Games
  games: ['games'] as const,
  game: (id: string) => ['games', id] as const,
  gameByName: (name: string) => ['games', 'by-name', name] as const,

  // Teams
  team: (id: string) => ['team', id] as const,
  inviteLink: (teamId: string) => ['team', teamId, 'invite'] as const,

  // Programs
  programs: ['programs'] as const,
  program: (id: string) => ['programs', id] as const,
  programVersions: (teamId: string, gameId: string) =>
    ['programs', 'versions', teamId, gameId] as const,

  // Admin / system
  queueStats: ['admin', 'queue-stats'] as const,
  matchStatistics: (tournamentId?: string) => ['admin', 'match-stats', tournamentId ?? 'all'] as const,
  systemMetrics: ['admin', 'system-metrics'] as const,
  systemHealth: ['admin', 'system-health'] as const,
  failedMatches: ['admin', 'failed-matches'] as const,
  matches: (limit: number, offset: number) => ['matches', limit, offset] as const,
};
