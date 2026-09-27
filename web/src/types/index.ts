// User types
export interface User {
  id: string;
  username: string;
  email: string;
  role: 'user' | 'admin';
  created_at: string;
  updated_at: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  refresh_token: string;
}

// Tournament types
export type TournamentStatus = 'pending' | 'active' | 'completed';

export interface Tournament {
  id: string;
  name: string;
  code: string;
  description: string;
  game_type: string;
  status: TournamentStatus;
  max_participants?: number;
  max_team_size: number;
  is_permanent: boolean;
  start_time?: string;
  end_time?: string;
  creator_id?: string;
  created_at: string;
  updated_at: string;
}

// Team types
export interface Team {
  id: string;
  tournament_id: string;
  name: string;
  leader_id: string;
  is_disqualified: boolean;
  disqualified_at: string | null;
  created_at: string;
  updated_at: string;
}

// TeamWithMembers - команда с участниками (поля Team встроены напрямую)
export interface TeamWithMembers extends Team {
  members: User[];
}

// Game types
export interface Game {
  id: string;
  name: string;
  display_name: string;
  rules: string;
  created_at: string;
  updated_at: string;
}

// TournamentGame - связь турнира с игрой со статусом раунда
export interface TournamentGameWithDetails {
  tournament_id: string;
  game_id: string;
  game_name: string;
  game_display_name: string;
  is_active: boolean;
  round_completed: boolean;
  round_completed_at?: string;
  auto_round_enabled: boolean;
  auto_round_interval_seconds: number;
  auto_round_last_run_at?: string;
  /** Почему авто-раунд не стартовал на последней проверке планировщика; нет - причины нет. */
  auto_round_wait?: 'matches_running' | 'interval' | 'new_programs' | 'participants';
}

// Program types
export interface Program {
  id: string;
  user_id: string;
  team_id?: string;
  tournament_id?: string;
  game_id?: string;
  name: string;
  game_type: string;
  language: string;
  /** Жизненный цикл: compiling - собирается в песочнице, ready - готова к матчам, failed - ошибка компиляции */
  status: 'compiling' | 'ready' | 'failed';
  error_message?: string;
  version: number;
  created_at: string;
  updated_at: string;
  /** Самопроверка против эталонного бота игры, только предупреждение; нет - проверки не было. */
  check_status?: 'pending' | 'ok' | 'failed' | null;
  /** Счёт с эталоном при ok, текст ошибки с хвостом stderr при failed; видит только своя команда. */
  check_message?: string | null;
}

// Match types
export type MatchStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';

export interface Match {
  id: string;
  tournament_id: string;
  program1_id: string;
  program2_id: string;
  game_type: string;
  status: MatchStatus;
  round_number: number;
  score1?: number;
  score2?: number;
  winner?: number;
  error_code?: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  /** Команды сторон; нет - команда удалена. */
  team1_id?: string | null;
  team1_name?: string | null;
  team2_id?: string | null;
  team2_name?: string | null;
}

// MatchRound - группа матчей одного раунда для конкретной игры
export interface MatchRound {
  round_number: number;
  game_type: string;
  total_matches: number;
  completed_count: number;
  pending_count: number;
  running_count: number;
  failed_count: number;
  wins1: number;
  wins2: number;
  // только в ответе на конкретный раунд, страницей
  matches?: Match[];
  created_at: string;
}

// Dry-run запуска раунда игры (GET /tournaments/{id}/run-game-matches/preview)
export interface RoundPreview {
  game_type: string;
  /** Матчи недоигранного раунда: запуск только вернёт их в очередь. */
  pending: number;
  /** Команды с готовой программой, играющие новый раунд. */
  participants: number;
  matches_created: number;
  /** Матчи прошлого раунда, удалятся вместе с историей рейтинга. */
  matches_deleted: number;
}

// Leaderboard types
export interface LeaderboardEntry {
  rank: number;
  program_id: string;
  program_name: string;
  team_id?: string;
  team_name?: string;
  username?: string;
  rating: number;
  wins: number;
  losses: number;
  draws: number;
  total_games: number;
}

// Cross-game leaderboard types
export interface GameRatingInfo {
  game_id: string;
  game_name: string;
  rating: number;
  wins: number;
  losses: number;
  draws: number;
  total_games: number;
}

export interface CrossGameLeaderboardEntry {
  rank: number;
  team_id?: string;
  team_name: string;
  program_id: string;
  program_name: string;
  game_ratings: Record<string, GameRatingInfo>;
  total_rating: number;
  total_wins: number;
  total_losses: number;
  total_games: number;
}

// API response types
// Тело ошибки бэкенда (handlers/responses.go writeError)
export interface ApiError {
  error: string;
}

// Queue stats types
export interface QueueStats {
  high: number;
  medium: number;
  low: number;
  total: number;
}

// Match statistics types
export interface MatchStatistics {
  total: number;
  pending: number;
  running: number;
  completed: number;
  failed: number;
}

// System metrics types
export interface CPUMetrics {
  usage_percent: number;
  cores: number;
  model_name?: string;
  per_core?: number[];
}

export interface MemoryMetrics {
  total: number;
  used: number;
  free: number;
  used_percent: number;
}

export interface DiskMetrics {
  total: number;
  used: number;
  free: number;
  used_percent: number;
  path: string;
}

export interface HostMetrics {
  hostname: string;
  platform: string;
  platform_version: string;
  os: string;
  arch: string;
  uptime: number;
}

export interface GoMetrics {
  version: string;
  goroutines: number;
  heap_alloc: number;
  heap_sys: number;
  num_gc: number;
  gomaxprocs: number;
}

export interface TemperatureInfo {
  sensor_key: string;
  temperature: number;
}

// Полное состояние системы (GET /system/status, admin)
export interface FullSystemStatus {
  app: {
    version: string;
    build_time: string;
    dirty: boolean;
    go_version: string;
    started_at: string;
    uptime_seconds: number;
  };
  database: {
    healthy: boolean;
    schema_version: number;
    schema_dirty: boolean;
    open_connections: number;
    in_use: number;
    idle: number;
    max_open: number;
  };
  redis: { healthy: boolean };
  queues: {
    high: number;
    medium: number;
    low: number;
    total: number;
    dead_letter: number;
    compile: number;
  };
  matches: {
    by_status: Record<string, number>;
    /** Матчи в running дольше WORKER_TIMEOUT+30с — чинится кнопкой восстановления */
    stuck_running: number;
    last_completed_at?: string | null;
  };
  programs: Record<string, number>;
  outbox?: {
    pending: number;
    errors: number;
    done_last_24h: number;
    oldest_pending_age_seconds?: number | null;
    last_processed_at?: string | null;
  } | null;
  websocket: { tournaments?: number; total_clients?: number };
}

export interface SystemMetrics {
  cpu: CPUMetrics;
  memory: MemoryMetrics;
  disk: DiskMetrics;
  host: HostMetrics;
  go: GoMetrics;
  temperature?: TemperatureInfo[];
}

// WebSocket message types
export interface WSMessage {
  type: string;
  payload: unknown;
}

// Head-to-head матрица: агрегат личных встреч пары команд в игре турнира
// (обе ориентации матчей AB/BA уже слиты бэкендом).
export interface HeadToHeadCell {
  team_id: string;
  team_name: string;
  opponent_id: string;
  opponent_name: string;
  wins: number;
  losses: number;
  draws: number;
  score_for: number;
  score_against: number;
}

// Ходы матча по итерациям из вывода tjudge-cli -v: [программа 1, программа 2].
// Дилемма: 1 - сотрудничать, 0 - предать; аукцион: ставки по очереди, 0 - пас.
// У упавшего матча ходы обрываются на ошибке, стороны бывают разной длины.
export interface MatchTranscript {
  moves: number[][];
  /** Очки сторон за каждую итерацию; у аукциона нет. */
  points?: number[][];
}

// Свойства стратегии команды в дилемме по Аксельроду: доли 0..1,
// null - ситуация ни разу не возникла.
export interface StrategyProfile {
  team_id: string;
  team_name: string;
  matches: number;
  cooperation: number | null;
  niceness: number | null;
  retaliation: number | null;
  forgiveness: number | null;
  provocability: number | null;
}

// Точка истории рейтинга программы (хронологический порядок).
export interface RatingHistoryPoint {
  id: string;
  program_id: string;
  tournament_id: string;
  old_rating: number;
  new_rating: number;
  change: number;
  match_id?: string;
  created_at: string;
}
