import axios, { type AxiosInstance, type AxiosError } from 'axios';
import type {
  User,
  AuthResponse,
  Tournament,
  Team,
  TeamWithMembers,
  Game,
  TournamentGameWithDetails,
  Program,
  Match,
  MatchRound,
  LeaderboardEntry,
  CrossGameLeaderboardEntry,
  ApiError,
  QueueStats,
  MatchStatistics,
  SystemMetrics,
} from '../types';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

class ApiClient {
  private client: AxiosInstance;
  private accessToken: string | null = null;

  // Mutex for token refresh to prevent race conditions
  private refreshPromise: Promise<void> | null = null;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Load token from localStorage
    this.accessToken = localStorage.getItem('access_token');

    // Request interceptor to add auth header
    this.client.interceptors.request.use((config) => {
      if (this.accessToken) {
        config.headers.Authorization = `Bearer ${this.accessToken}`;
      }
      return config;
    });

    // Response interceptor for error handling
    this.client.interceptors.response.use(
      (response) => response,
      async (error: AxiosError<ApiError>) => {
        const originalRequest = error.config;

        // Skip refresh logic for auth endpoints (they don't need token refresh):
        // - /auth/refresh: would cause infinite loop
        // - /auth/logout: user is logging out, no need to refresh
        // - /auth/login: user is authenticating, no token to refresh
        // - /auth/register: new user registration, no token exists
        // Also skip if request was already retried or no config exists
        const requestWithRetry = originalRequest as unknown as { _retry?: boolean };
        const isAuthEndpoint = originalRequest?.url?.includes('/auth/');
        if (
          error.response?.status === 401 &&
          originalRequest &&
          !isAuthEndpoint &&
          !requestWithRetry._retry
        ) {
          requestWithRetry._retry = true;

          // Use mutex to prevent concurrent refresh attempts
          try {
            await this.refreshTokenWithMutex();
            // Retry original request with new token
            return this.client.request(originalRequest);
          } catch {
            // Refresh failed - just clear tokens locally, don't call logout API
            // (calling logout API would cause another 401 and infinite loop)
            this.clearTokens();
            // Redirect to login page
            window.location.href = '/login';
          }
        }
        return Promise.reject(error);
      }
    );
  }

  /**
   * Refresh token with mutex to prevent race conditions.
   * Multiple concurrent 401 errors will all wait for the same refresh promise.
   */
  private async refreshTokenWithMutex(): Promise<void> {
    // If a refresh is already in progress, wait for it
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    // Start a new refresh and store the promise
    this.refreshPromise = this.refreshToken()
      .finally(() => {
        // Clear the promise when done (success or failure)
        this.refreshPromise = null;
      });

    return this.refreshPromise;
  }

  setAccessToken(token: string) {
    this.accessToken = token;
    localStorage.setItem('access_token', token);
  }

  clearTokens() {
    this.accessToken = null;
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
  }

  // Auth endpoints
  async register(username: string, email: string, password: string): Promise<AuthResponse> {
    const { data } = await this.client.post<AuthResponse>('/auth/register', {
      username,
      email,
      password,
    });
    this.setAccessToken(data.access_token);
    localStorage.setItem('refresh_token', data.refresh_token);
    return data;
  }

  async login(username: string, password: string): Promise<AuthResponse> {
    const { data } = await this.client.post<AuthResponse>('/auth/login', {
      username,
      password,
    });
    this.setAccessToken(data.access_token);
    localStorage.setItem('refresh_token', data.refresh_token);
    return data;
  }

  async refreshToken(): Promise<void> {
    const refreshToken = localStorage.getItem('refresh_token');
    if (!refreshToken) throw new Error('No refresh token');

    const { data } = await this.client.post<AuthResponse>('/auth/refresh', {
      refresh_token: refreshToken,
    });
    this.setAccessToken(data.access_token);
    localStorage.setItem('refresh_token', data.refresh_token);
  }

  async logout(): Promise<void> {
    try {
      const refreshToken = localStorage.getItem('refresh_token');
      // Send both tokens for proper invalidation
      await this.client.post('/auth/logout', { refresh_token: refreshToken });
    } finally {
      this.clearTokens();
    }
  }

  async getMe(): Promise<User> {
    const { data } = await this.client.get<User>('/auth/me');
    return data;
  }

  async updateProfile(updates: { email?: string; password?: string }): Promise<User> {
    const { data } = await this.client.put<User>('/auth/profile', updates);
    return data;
  }

  // Tournament endpoints
  async getTournaments(status?: string): Promise<Tournament[]> {
    const params = status ? { status } : {};
    const { data } = await this.client.get<Tournament[]>('/tournaments', { params });
    return data;
  }

  async getTournament(id: string): Promise<Tournament> {
    const { data } = await this.client.get<Tournament>(`/tournaments/${id}`);
    return data;
  }

  async createTournament(tournament: Partial<Tournament>): Promise<Tournament> {
    const { data } = await this.client.post<Tournament>('/tournaments', tournament);
    return data;
  }

  async joinTournament(id: string, programId: string): Promise<void> {
    await this.client.post(`/tournaments/${id}/join`, { program_id: programId });
  }

  async startTournament(id: string): Promise<void> {
    await this.client.post(`/tournaments/${id}/start`);
  }

  async completeTournament(id: string): Promise<void> {
    await this.client.post(`/tournaments/${id}/complete`);
  }

  async deleteTournament(id: string): Promise<void> {
    await this.client.delete(`/tournaments/${id}`);
  }

  async getLeaderboard(tournamentId: string, limit = 100): Promise<LeaderboardEntry[]> {
    const { data } = await this.client.get<LeaderboardEntry[]>(
      `/tournaments/${tournamentId}/leaderboard`,
      { params: { limit } }
    );
    return data;
  }

  async getCrossGameLeaderboard(tournamentId: string): Promise<CrossGameLeaderboardEntry[]> {
    const { data } = await this.client.get<CrossGameLeaderboardEntry[]>(
      `/tournaments/${tournamentId}/cross-game-leaderboard`
    );
    return data;
  }

  async runAllMatches(tournamentId: string): Promise<{ status: string; enqueued: number }> {
    const { data } = await this.client.post<{ status: string; enqueued: number }>(
      `/tournaments/${tournamentId}/run-matches`
    );
    return data;
  }

  async retryFailedMatches(tournamentId: string): Promise<{ status: string; enqueued: number }> {
    const { data } = await this.client.post<{ status: string; enqueued: number }>(
      `/tournaments/${tournamentId}/retry-matches`
    );
    return data;
  }

  async runGameMatches(tournamentId: string, gameType: string): Promise<{ status: string; game_type: string; enqueued: number }> {
    const { data } = await this.client.post<{ status: string; game_type: string; enqueued: number }>(
      `/tournaments/${tournamentId}/run-game-matches`,
      { game_type: gameType }
    );
    return data;
  }

  async getTournamentMatches(tournamentId: string, limit = 50, offset = 0): Promise<Match[]> {
    const { data } = await this.client.get<Match[]>(`/tournaments/${tournamentId}/matches`, {
      params: { limit, offset },
    });
    return data;
  }

  async getMatchesByRounds(tournamentId: string): Promise<MatchRound[]> {
    const { data } = await this.client.get<MatchRound[]>(
      `/tournaments/${tournamentId}/matches/rounds`
    );
    return data;
  }

  async getMyTeam(tournamentId: string): Promise<Team | null> {
    const { data } = await this.client.get<Team | null>(`/tournaments/${tournamentId}/my-team`);
    return data;
  }

  async getTournamentTeams(tournamentId: string): Promise<Team[]> {
    const { data } = await this.client.get<Team[]>(`/tournaments/${tournamentId}/teams`);
    return data;
  }

  async getTournamentGames(tournamentId: string): Promise<Game[]> {
    const { data } = await this.client.get<Game[]>(`/tournaments/${tournamentId}/games`);
    return data;
  }

  async getTournamentGamesStatus(tournamentId: string): Promise<TournamentGameWithDetails[]> {
    const { data } = await this.client.get<TournamentGameWithDetails[]>(
      `/tournaments/${tournamentId}/games/status`
    );
    return data;
  }

  async markGameRoundCompleted(tournamentId: string, gameId: string): Promise<void> {
    await this.client.post(`/tournaments/${tournamentId}/games/${gameId}/complete-round`);
  }

  async setActiveGame(tournamentId: string, gameId: string): Promise<void> {
    await this.client.post(`/tournaments/${tournamentId}/active-game`, { game_id: gameId });
  }

  async getActiveGame(tournamentId: string): Promise<TournamentGameWithDetails | null> {
    const { data } = await this.client.get<TournamentGameWithDetails | null>(
      `/tournaments/${tournamentId}/active-game`
    );
    return data;
  }

  async resetGameRound(tournamentId: string, gameId: string): Promise<{
    matches_deleted: number;
    participants_reset: number;
    rating_history_reset: number;
  }> {
    const { data } = await this.client.post(
      `/tournaments/${tournamentId}/games/${gameId}/reset-round`
    );
    return data;
  }

  // Team endpoints
  async createTeam(tournamentId: string, name: string): Promise<Team> {
    const { data } = await this.client.post<Team>('/teams', { tournament_id: tournamentId, name });
    return data;
  }

  async joinTeamByCode(code: string): Promise<Team> {
    const { data } = await this.client.post<Team>('/teams/join', { code });
    return data;
  }

  async getTeam(id: string): Promise<TeamWithMembers> {
    const { data } = await this.client.get<TeamWithMembers>(`/teams/${id}`);
    return data;
  }

  async updateTeamName(id: string, name: string): Promise<Team> {
    const { data } = await this.client.put<Team>(`/teams/${id}`, { name });
    return data;
  }

  async leaveTeam(id: string): Promise<void> {
    await this.client.post(`/teams/${id}/leave`);
  }

  async removeMember(teamId: string, userId: string): Promise<void> {
    await this.client.delete(`/teams/${teamId}/members/${userId}`);
  }

  async getInviteLink(teamId: string): Promise<{ code: string; link: string }> {
    const { data } = await this.client.get<{ code: string; link: string }>(
      `/teams/${teamId}/invite`
    );
    return data;
  }

  // Game endpoints
  async getGames(): Promise<Game[]> {
    const { data } = await this.client.get<Game[]>('/games');
    return data;
  }

  async getGame(id: string): Promise<Game> {
    const { data } = await this.client.get<Game>(`/games/${id}`);
    return data;
  }

  async getGameByName(name: string): Promise<Game> {
    const { data } = await this.client.get<Game>(`/games/name/${name}`);
    return data;
  }

  async createGame(game: { name: string; display_name: string; rules: string }): Promise<Game> {
    const { data } = await this.client.post<Game>('/games', game);
    return data;
  }

  async updateGame(
    id: string,
    game: { display_name: string; rules: string }
  ): Promise<Game> {
    const { data } = await this.client.put<Game>(`/games/${id}`, game);
    return data;
  }

  async deleteGame(id: string): Promise<void> {
    await this.client.delete(`/games/${id}`);
  }

  async addGameToTournament(tournamentId: string, gameId: string): Promise<void> {
    await this.client.post(`/tournaments/${tournamentId}/games`, { game_id: gameId });
  }

  async removeGameFromTournament(tournamentId: string, gameId: string): Promise<void> {
    await this.client.delete(`/tournaments/${tournamentId}/games/${gameId}`);
  }

  async getGameLeaderboard(tournamentId: string, gameId: string, limit = 100): Promise<LeaderboardEntry[]> {
    const { data } = await this.client.get<LeaderboardEntry[]>(
      `/tournaments/${tournamentId}/games/${gameId}/leaderboard`,
      { params: { limit } }
    );
    return data;
  }

  async getGameMatches(
    tournamentId: string,
    gameId: string,
    status?: string,
    limit = 50,
    offset = 0
  ): Promise<Match[]> {
    const params: Record<string, unknown> = { limit, offset };
    if (status) params.status = status;
    const { data } = await this.client.get<Match[]>(
      `/tournaments/${tournamentId}/games/${gameId}/matches`,
      { params }
    );
    return data;
  }

  async getGamePrograms(tournamentId: string, gameId: string): Promise<Program[]> {
    const { data } = await this.client.get<Program[]>(
      `/tournaments/${tournamentId}/games/${gameId}/programs`
    );
    return data;
  }

  // Program endpoints
  async getPrograms(): Promise<Program[]> {
    const { data } = await this.client.get<Program[]>('/programs');
    return data;
  }

  async getProgram(id: string): Promise<Program> {
    const { data } = await this.client.get<Program>(`/programs/${id}`);
    return data;
  }

  async uploadProgram(formData: FormData): Promise<Program> {
    const { data } = await this.client.post<Program>('/programs', formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
    });
    return data;
  }

  async deleteProgram(id: string): Promise<void> {
    await this.client.delete(`/programs/${id}`);
  }

  async downloadProgram(id: string): Promise<Blob> {
    const { data } = await this.client.get<Blob>(`/programs/${id}/download`, {
      responseType: 'blob',
    });
    return data;
  }

  async getProgramVersions(teamId: string, gameId: string): Promise<Program[]> {
    const { data } = await this.client.get<Program[]>('/programs/versions', {
      params: { team_id: teamId, game_id: gameId },
    });
    return data;
  }

  async deleteTeam(id: string): Promise<void> {
    await this.client.delete(`/teams/${id}`);
  }

  // Match endpoints
  async getMatches(limit = 50, offset = 0): Promise<Match[]> {
    const { data } = await this.client.get<Match[]>('/matches', {
      params: { limit, offset },
    });
    return data;
  }

  async getMatch(id: string): Promise<Match> {
    const { data } = await this.client.get<Match>(`/matches/${id}`);
    return data;
  }

  // System endpoints (admin only)
  async getQueueStats(): Promise<QueueStats> {
    const { data } = await this.client.get<QueueStats>('/matches/queue/stats');
    return data;
  }

  async getMatchStatistics(tournamentId?: string): Promise<MatchStatistics> {
    const params = tournamentId ? { tournament_id: tournamentId } : {};
    const { data } = await this.client.get<MatchStatistics>('/matches/statistics', { params });
    return data;
  }

  async clearQueue(): Promise<{ message: string }> {
    const { data } = await this.client.post<{ message: string }>('/matches/queue/clear');
    return data;
  }

  async purgeInvalidMatches(): Promise<{ message: string; purged_count: number }> {
    const { data } = await this.client.post<{ message: string; purged_count: number }>('/matches/queue/purge');
    return data;
  }

  // System endpoints (admin only)
  async getSystemMetrics(): Promise<SystemMetrics> {
    const { data } = await this.client.get<SystemMetrics>('/system/metrics');
    return data;
  }

  async getSystemHealth(): Promise<{ status: string; timestamp: string; hostname: string; pid: number }> {
    const { data } = await this.client.get<{ status: string; timestamp: string; hostname: string; pid: number }>('/system/health');
    return data;
  }

  // Get failed matches (for admin error display)
  async getFailedMatches(limit: number = 20): Promise<Match[]> {
    const { data } = await this.client.get<Match[]>('/matches', {
      params: { status: 'failed', limit: limit.toString() }
    });
    return data;
  }
}

export const api = new ApiClient();
export default api;
