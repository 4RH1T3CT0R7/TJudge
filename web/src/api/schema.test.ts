// P2.13: тесты schema-валидатора (vitest-совместимые).
// Исполняются при наличии `npm test` — нет runner'а в проекте сейчас, но файл
// готов для активации. Паттерн: проверка happy-path + каждой выбрасываемой ошибки.

import {
  validateUser,
  validateAuthResponse,
  validateTournament,
  validateTournamentList,
  validateGame,
  SchemaError,
} from './schema';

function expectThrows(fn: () => void, pathContains: string) {
  try {
    fn();
    throw new Error('expected SchemaError but got none');
  } catch (e) {
    if (!(e instanceof SchemaError)) throw e;
    if (!e.path.includes(pathContains)) {
      throw new Error(`expected path to contain "${pathContains}", got "${e.path}"`);
    }
  }
}

// Smoke tests — экспортируются чтобы runner'у было что подхватить.
export function runAllTests() {
  // user
  validateUser({ id: '1', username: 'u', email: 'e', role: 'user' });
  expectThrows(() => validateUser(null), 'user');
  expectThrows(() => validateUser({ id: 1, username: 'u', email: 'e', role: 'user' }), 'user.id');
  expectThrows(() => validateUser({ id: '1', username: 'u', email: 'e' }), 'user.role');

  // authResponse
  validateAuthResponse({
    access_token: 'a',
    refresh_token: 'r',
    user: { id: '1', username: 'u', email: 'e', role: 'user' },
  });
  expectThrows(() => validateAuthResponse({ access_token: 'a' }), 'authResponse');

  // tournament
  validateTournament({ id: '1', name: 'T', status: 'pending' });
  expectThrows(() => validateTournament({ id: '1', name: 'T' }), 'tournament.status');

  // tournamentList
  validateTournamentList([
    { id: '1', name: 'A', status: 'pending' },
    { id: '2', name: 'B', status: 'active' },
  ]);
  expectThrows(
    () => validateTournamentList([{ id: '1', name: 'A' }]),
    'tournamentList[0]'
  );

  // game
  validateGame({ id: '1', name: 'n', display_name: 'Display' });
  expectThrows(() => validateGame({ id: '1' }), 'game.name');
}
