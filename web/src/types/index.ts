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
  code: string;
  leader_id: string;
  is_disqualified: boolean;
  disqualified_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface TeamMember {
  id: string;
  team_id: string;
  user_id: string;
  joined_at: string;
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
  current_round: number;
  auto_round_enabled: boolean;
  auto_round_interval_seconds: number;
  auto_round_last_run_at?: string;
}

// Program types
export type ProgramStatus = 'pending' | 'compiling' | 'ready' | 'error';

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
}

// Match types
export type MatchStatus = 'pending' | 'running' | 'completed' | 'failed';

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
  matches: Match[];
  created_at: string;
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
export interface ApiError {
  code: number;
  message: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
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
    /** Матчи в running дольше 2 минут — чинится кнопкой восстановления */
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

export interface LeaderboardUpdate {
  entries: LeaderboardEntry[];
}

export interface MatchUpdate {
  match: Match;
}

export interface TournamentUpdate {
  status: TournamentStatus;
  matches_count?: number;
  start_time?: string;
  end_time?: string;
}
