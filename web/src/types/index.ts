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
  code_path: string;
  file_path?: string;
  language: string;
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
  score1?: number;
  score2?: number;
  winner?: number;
  error_code?: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
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
