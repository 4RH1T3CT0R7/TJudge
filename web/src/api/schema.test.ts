// Тесты schema-валидатора (vitest).
// Паттерн: проверка happy-path + каждой выбрасываемой ошибки.

import { describe, it, expect } from 'vitest';
import {
  validateUser,
  validateAuthResponse,
  validateTournament,
  validateTournamentList,
  validateGame,
  SchemaError,
} from './schema';

function expectSchemaError(fn: () => void, pathContains: string) {
  try {
    fn();
    throw new Error('expected SchemaError but got none');
  } catch (e) {
    expect(e).toBeInstanceOf(SchemaError);
    expect((e as SchemaError).path).toContain(pathContains);
  }
}

describe('validateUser', () => {
  it('принимает валидного пользователя', () => {
    expect(() => validateUser({ id: '1', username: 'u', email: 'e', role: 'user' })).not.toThrow();
  });

  it('отклоняет null', () => {
    expectSchemaError(() => validateUser(null), 'user');
  });

  it('отклоняет числовой id', () => {
    expectSchemaError(() => validateUser({ id: 1, username: 'u', email: 'e', role: 'user' }), 'user.id');
  });

  it('отклоняет отсутствие role', () => {
    expectSchemaError(() => validateUser({ id: '1', username: 'u', email: 'e' }), 'user.role');
  });
});

describe('validateAuthResponse', () => {
  it('принимает валидный ответ', () => {
    expect(() =>
      validateAuthResponse({
        access_token: 'a',
        refresh_token: 'r',
        user: { id: '1', username: 'u', email: 'e', role: 'user' },
      })
    ).not.toThrow();
  });

  it('отклоняет ответ без user/refresh_token', () => {
    expectSchemaError(() => validateAuthResponse({ access_token: 'a' }), 'authResponse');
  });
});

describe('validateTournament', () => {
  it('принимает валидный турнир', () => {
    expect(() => validateTournament({ id: '1', name: 'T', status: 'pending' })).not.toThrow();
  });

  it('отклоняет турнир без статуса', () => {
    expectSchemaError(() => validateTournament({ id: '1', name: 'T' }), 'tournament.status');
  });
});

describe('validateTournamentList', () => {
  it('принимает валидный список', () => {
    expect(() =>
      validateTournamentList([
        { id: '1', name: 'A', status: 'pending' },
        { id: '2', name: 'B', status: 'active' },
      ])
    ).not.toThrow();
  });

  it('указывает индекс сломанного элемента', () => {
    expectSchemaError(() => validateTournamentList([{ id: '1', name: 'A' }]), 'tournamentList[0]');
  });
});

describe('validateGame', () => {
  it('принимает валидную игру', () => {
    expect(() => validateGame({ id: '1', name: 'n', display_name: 'Display' })).not.toThrow();
  });

  it('отклоняет игру без name', () => {
    expectSchemaError(() => validateGame({ id: '1' }), 'game.name');
  });
});
