import { describe, it, expect } from 'vitest';
import { parseTournamentWSMessage } from './ws';

describe('parseTournamentWSMessage', () => {
  it('распознаёт все известные типы сообщений', () => {
    for (const type of ['tournament_update', 'matches_created', 'match_result', 'program_update']) {
      const msg = parseTournamentWSMessage({ type, payload: {} });
      expect(msg).not.toBeNull();
      expect(msg?.type).toBe(type);
    }
  });

  it('возвращает null для неизвестного типа (forward-compat)', () => {
    expect(parseTournamentWSMessage({ type: 'future_event', payload: {} })).toBeNull();
  });

  it('сохраняет payload без изменений', () => {
    const payload = { program_id: 'p1', team_id: 't1', status: 'ready', error_message: null };
    const msg = parseTournamentWSMessage({ type: 'program_update', payload });
    expect(msg?.payload).toEqual(payload);
  });
});
