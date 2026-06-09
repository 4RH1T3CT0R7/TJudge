import axios, { type AxiosInstance, type AxiosError } from 'axios';
import type {
  FullSystemStatus,
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
import { useToastStore } from '../store/toastStore';
import {
  validateAuthResponse,
  validateUser,
  validateTournament,
  validateTournamentList,
  validateGame,
  SchemaError,
} from './schema';

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

// Retry/backoff parameters для transient-ошибок (5xx, network).
const MAX_RETRY_ATTEMPTS = 3;
const BASE_RETRY_DELAY_MS = 300;

// Маппинг HTTP-статусов в user-friendly русские сообщения.
// Используется в response interceptor для toast-уведомлений.
function humanErrorMessage(
  status: number | undefined,
  raw: string | null | undefined
): string {
  if (raw && typeof raw === 'string' && raw.length > 0) {
    return raw;
  }
  switch (status) {
    case 400:
      return 'Некорректный запрос. Проверьте введённые данные.';
    case 403:
      return 'Доступ запрещён — недостаточно прав.';
    case 404:
      return 'Запрашиваемый ресурс не найден.';
    case 409:
      return 'Конфликт — данные изменены в другом месте.';
    case 413:
      return 'Файл слишком большой.';
    case 422:
      return 'Некорректные данные.';
    case 429:
      return 'Слишком много запросов, попробуйте позже.';
    case 500:
    case 502:
    case 503:
    case 504:
      return 'Ошибка сервера. Попробуйте повторить через несколько секунд.';
    default:
      if (!status) return 'Сеть недоступна. Проверьте соединение.';
      return `Произошла ошибка (код ${status}).`;
  }
}

// isRetryableError возвращает true для transient-ошибок, где retry имеет смысл.
function isRetryableError(error: AxiosError): boolean {
  // Network / timeout без response - retry полезен.
  if (!error.response) return true;
  const status = error.response.status;
  // 5xx и 429 - сервер перегружен/временно недоступен.
  return status >= 500 || status === 429;
}

class ApiClient {
  private client: AxiosInstance;
  private accessToken: string | null = null;

  // Mutex для refresh токена, чтобы избежать гонок
  private refreshPromise: Promise<void> | null = null;

  // Callback для auth failure (SPA-friendly редирект без full page reload)
  private onAuthFailure: (() => void) | null = null;

  constructor() {
    this.client = axios.create({
      baseURL: API_BASE_URL,
      headers: {
        'Content-Type': 'application/json',
      },
    });

    // Загружаем токен из localStorage
    this.accessToken = localStorage.getItem('access_token');

    // Request interceptor для добавления auth header
    this.client.interceptors.request.use((config) => {
      if (this.accessToken) {
        config.headers.Authorization = `Bearer ${this.accessToken}`;
      }
      return config;
    });

    // Response interceptor: разворачиваем стандартный API envelope и обрабатываем ошибки.
    // Бэкенд оборачивает все ответы в { data, message?, meta? }.
    // Этот interceptor извлекает внутреннее поле `data`, чтобы вызывающий код получал
    // payload напрямую (например, response.data это Tournament[], а не { data: Tournament[] }).
    this.client.interceptors.response.use(
      (response) => {
        // Разворачиваем только JSON-ответы со стандартным envelope { data, message?, meta? }.
        const contentType = String(response.headers['content-type'] || '');
        if (
          contentType.includes('application/json') &&
          response.data &&
          typeof response.data === 'object' &&
          !Array.isArray(response.data) &&
          'data' in response.data
        ) {
          response.data = response.data.data;
        }
        return response;
      },
      async (error: AxiosError<ApiError>) => {
        const originalRequest = error.config;

        // Пропускаем refresh для auth-эндпоинтов (им не нужен refresh токена):
        // - /auth/refresh: вызвал бы бесконечный цикл
        // - /auth/logout: пользователь выходит, refresh не нужен
        // - /auth/login: пользователь аутентифицируется, токена ещё нет
        // - /auth/register: регистрация нового пользователя, токена не существует
        // Также пропускаем, если запрос уже повторён или нет config
        const requestWithRetry = originalRequest as unknown as {
          _retry?: boolean;
          _retryCount?: number;
        };
        const isAuthEndpoint = originalRequest?.url?.includes('/auth/');
        if (
          error.response?.status === 401 &&
          originalRequest &&
          !isAuthEndpoint &&
          !requestWithRetry._retry
        ) {
          requestWithRetry._retry = true;

          // Mutex предотвращает одновременные refresh-попытки
          try {
            await this.refreshTokenWithMutex();
            // Повторяем исходный запрос с новым токеном
            return this.client.request(originalRequest);
          } catch {
            // Refresh провалился - просто очищаем токены локально, не вызываем logout API
            // (вызов logout API привёл бы к ещё одному 401 и бесконечному циклу)
            this.clearTokens();
            // Уведомляем подписчиков (напр. auth store), чтобы React Router сделал navigate
            if (this.onAuthFailure) {
              this.onAuthFailure();
            }
          }
        }

        // Exponential backoff retry для transient-ошибок (5xx, 429, network).
        // Идемпотентность: повторяем только GET/HEAD/OPTIONS, иначе можно создать дубль.
        const method = originalRequest?.method?.toUpperCase() || 'GET';
        const safeMethod = method === 'GET' || method === 'HEAD' || method === 'OPTIONS';
        if (
          originalRequest &&
          safeMethod &&
          isRetryableError(error) &&
          !isAuthEndpoint
        ) {
          const attempt = (requestWithRetry._retryCount || 0) + 1;
          if (attempt <= MAX_RETRY_ATTEMPTS) {
            requestWithRetry._retryCount = attempt;
            const delay = BASE_RETRY_DELAY_MS * Math.pow(2, attempt - 1);
            await new Promise((resolve) => setTimeout(resolve, delay));
            return this.client.request(originalRequest);
          }
        }

        // Показываем глобальный error-toast для не-401 ошибок
        // (401 обрабатываются логикой refresh токена выше)
        if (error.response?.status !== 401) {
          const responseData = error.response?.data as
            | Record<string, unknown>
            | undefined;
          const rawMessage =
            (typeof responseData?.error === 'string' ? responseData.error : null) ||
            (typeof responseData?.message === 'string' ? responseData.message : null);
          const message = humanErrorMessage(error.response?.status, rawMessage);
          useToastStore.getState().addToast(message, 'error');
        }

        return Promise.reject(error);
      }
    );
  }

  /**
   * Refresh токена с mutex для защиты от гонок.
   * Несколько одновременных 401 будут ждать один и тот же refresh-promise.
   */
  private async refreshTokenWithMutex(): Promise<void> {
    // Если refresh уже идёт - ждём его
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    // Запускаем новый refresh и сохраняем promise
    this.refreshPromise = this.refreshToken()
      .finally(() => {
        // Очищаем promise по завершению (успех или неудача)
        this.refreshPromise = null;
      });

    return this.refreshPromise;
  }

  setOnAuthFailure(callback: () => void) {
    this.onAuthFailure = callback;
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

  // Эндпоинты авторизации
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
    // Runtime-валидация - ранний сигнал при несовпадении схемы.
    this.validateOrWarn(() => validateAuthResponse(data), 'POST /auth/login');
    this.setAccessToken(data.access_token);
    localStorage.setItem('refresh_token', data.refresh_token);
    return data;
  }

  /**
   * validateOrWarn запускает schema-валидатор и логирует SchemaError как warning.
   * Не кидает наружу, чтобы не ломать UX - большинство мелких несоответствий
   * проявят себя в UI-ошибках ("undefined field"), но лог сразу покажет корень.
   */
  private validateOrWarn(fn: () => void, ctx: string) {
    try {
      fn();
    } catch (e) {
      if (e instanceof SchemaError) {
        console.warn(`[schema] ${ctx}: ${e.message}`);
      }
    }
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
    this.validateOrWarn(() => validateUser(data), 'GET /auth/me');
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
    this.validateOrWarn(() => validateTournamentList(data), 'GET /tournaments');
    return data;
  }

  async getTournament(id: string): Promise<Tournament> {
    const { data } = await this.client.get<Tournament>(`/tournaments/${id}`);
    this.validateOrWarn(() => validateTournament(data), 'GET /tournaments/{id}');
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

  async deactivateAllGames(tournamentId: string): Promise<void> {
    await this.client.post(`/tournaments/${tournamentId}/games/deactivate-all`);
  }

  async clearProgramErrors(tournamentId: string): Promise<{ cleared: number; message: string }> {
    const { data } = await this.client.post<{ cleared: number; message: string }>(
      `/tournaments/${tournamentId}/programs/clear-errors`
    );
    return data;
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

  async setAutoRound(
    tournamentId: string,
    gameId: string,
    enabled: boolean,
    intervalSeconds: number
  ): Promise<{ enabled: boolean; interval_seconds: number }> {
    const { data } = await this.client.post(
      `/tournaments/${tournamentId}/games/${gameId}/auto-round`,
      { enabled, interval_seconds: intervalSeconds }
    );
    return data;
  }

  async getAutoRound(
    tournamentId: string,
    gameId: string
  ): Promise<{ enabled: boolean; interval_seconds: number; last_run_at: string | null }> {
    const { data } = await this.client.get(
      `/tournaments/${tournamentId}/games/${gameId}/auto-round`
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
    this.validateOrWarn(() => validateGame(data), 'GET /games/{id}');
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

  async downloadTournamentPrograms(tournamentId: string): Promise<Blob> {
    const { data } = await this.client.get<Blob>(
      `/tournaments/${tournamentId}/programs/download-zip`,
      { responseType: 'blob' }
    );
    return data;
  }

  async getProgramVersions(teamId: string, gameId: string): Promise<Program[]> {
    const { data } = await this.client.get<Program[]>('/programs/versions', {
      params: { team_id: teamId, game_id: gameId },
    });
    return data;
  }

  async disqualifyTeam(teamId: string): Promise<{ matches_deleted: number; matches_cancelled: number; rating_history_reset: number }> {
    const { data } = await this.client.post(`/teams/${teamId}/disqualify`);
    return data;
  }

  async restoreTeam(teamId: string): Promise<void> {
    await this.client.post(`/teams/${teamId}/restore`);
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

  async getFullSystemStatus(): Promise<FullSystemStatus> {
    const { data } = await this.client.get<FullSystemStatus>('/system/status');
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
