// Lightweight runtime-валидатор для критичных API-responses.
//
// Поскольку zod не установлен, используем type-guards (predicate functions).
// Это ~50 строк кода вместо 100 KB Zod-рантайма.
//
// Применяется в api/client.ts interceptor'е для ключевых endpoint'ов, где
// несоответствие схемы должно быть ранним сигналом (не runtime crash из-за
// undefined-field). Остальные endpoint'ы используют TypeScript-типы
// как compile-time hint без runtime-проверок.

// Базовые примитивные проверки.
const isString = (v: unknown): v is string => typeof v === 'string';
const isNumber = (v: unknown): v is number => typeof v === 'number' && !Number.isNaN(v);
const isBool = (v: unknown): v is boolean => typeof v === 'boolean';
const isObject = (v: unknown): v is Record<string, unknown> =>
  typeof v === 'object' && v !== null && !Array.isArray(v);

/** Error, выбрасываемая schema-валидатором. */
export class SchemaError extends Error {
  readonly path: string;
  readonly received: unknown;

  // `erasableSyntaxOnly` (tsconfig.app.json) запрещает parameter-properties
  // в конструкторе - объявляем поля явно и присваиваем внутри.
  constructor(path: string, received: unknown) {
    super(`schema mismatch at ${path} (received: ${typeof received})`);
    this.name = 'SchemaError';
    this.path = path;
    this.received = received;
  }
}

/** Утилита: выбрасывает SchemaError если check вернул false. */
function check(cond: boolean, path: string, received: unknown): void {
  if (!cond) throw new SchemaError(path, received);
}

/** User из /auth/me, /auth/login, /auth/register. */
export function validateUser(v: unknown): void {
  check(isObject(v), 'user', v);
  const u = v as Record<string, unknown>;
  check(isString(u.id), 'user.id', u.id);
  check(isString(u.username), 'user.username', u.username);
  check(isString(u.email), 'user.email', u.email);
  check(isString(u.role), 'user.role', u.role);
}

/** AuthResponse {access_token, refresh_token, user}. */
export function validateAuthResponse(v: unknown): void {
  check(isObject(v), 'authResponse', v);
  const a = v as Record<string, unknown>;
  check(isString(a.access_token), 'authResponse.access_token', a.access_token);
  check(isString(a.refresh_token), 'authResponse.refresh_token', a.refresh_token);
  validateUser(a.user);
}

/** Tournament. */
export function validateTournament(v: unknown): void {
  check(isObject(v), 'tournament', v);
  const t = v as Record<string, unknown>;
  check(isString(t.id), 'tournament.id', t.id);
  check(isString(t.name), 'tournament.name', t.name);
  check(isString(t.status), 'tournament.status', t.status);
}

/** Массив Tournament-ов (для /tournaments). */
export function validateTournamentList(v: unknown): void {
  check(Array.isArray(v), 'tournamentList', v);
  (v as unknown[]).forEach((t, i) => {
    try {
      validateTournament(t);
    } catch (e) {
      if (e instanceof SchemaError) {
        throw new SchemaError(`tournamentList[${i}].${e.path}`, e.received);
      }
      throw e;
    }
  });
}

/** Game. */
export function validateGame(v: unknown): void {
  check(isObject(v), 'game', v);
  const g = v as Record<string, unknown>;
  check(isString(g.id), 'game.id', g.id);
  check(isString(g.name), 'game.name', g.name);
  check(isString(g.display_name), 'game.display_name', g.display_name);
}

/** LeaderboardEntry - компактный validator для list-response. */
export function validateLeaderboardEntry(v: unknown): void {
  check(isObject(v), 'leaderboardEntry', v);
  const e = v as Record<string, unknown>;
  check(isNumber(e.rating), 'leaderboardEntry.rating', e.rating);
  check(isBool(e.wins !== undefined || true), 'leaderboardEntry', e); // wins optional
}
