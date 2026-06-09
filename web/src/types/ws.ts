// Типизированные WebSocket-события.
//
// Источник истины - бэкенд: internal/events/handlers/broadcast.go
// (BroadcastHandler.Handle). При добавлении события на бэке - добавить
// payload-тип и ветку в discriminated union здесь.

export interface TournamentUpdatePayload {
  status: string;
  start_time?: string | null;
  end_time?: string | null;
}

export interface MatchesCreatedPayload {
  program_id: string;
  matches_count: number;
}

export interface MatchResultPayload {
  match_id: string;
  program1_id: string;
  program2_id: string;
  new_rating1: number;
  new_rating2: number;
  winner: number;
}

export interface ProgramUpdatePayload {
  program_id: string;
  team_id: string;
  status: 'compiling' | 'ready' | 'failed';
  error_message: string | null;
}

/** Discriminated union всех серверных WS-сообщений турнира. */
export type TournamentWSMessage =
  | { type: 'tournament_update'; payload: TournamentUpdatePayload }
  | { type: 'matches_created'; payload: MatchesCreatedPayload }
  | { type: 'match_result'; payload: MatchResultPayload }
  | { type: 'program_update'; payload: ProgramUpdatePayload };

/** Парсит сырое WS-сообщение в типизированное; null для неизвестных типов. */
export function parseTournamentWSMessage(raw: { type: string; payload: unknown }): TournamentWSMessage | null {
  switch (raw.type) {
    case 'tournament_update':
    case 'matches_created':
    case 'match_result':
    case 'program_update':
      return raw as TournamentWSMessage;
    default:
      return null;
  }
}
