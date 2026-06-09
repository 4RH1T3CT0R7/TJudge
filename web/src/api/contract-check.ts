// Контрактная проверка типов: рукописные типы фронта (src/types/index.ts)
// не должны расходиться со схемой API (docs/openapi.yaml).
//
// Сгенерированные типы: src/api/generated/openapi.d.ts
// (npm run generate:api; свежесть проверяется в CI).
//
// Проверка направленная: каждое поле рукописного типа обязано существовать
// в OpenAPI-схеме. Если фронт использует поле, которого нет в контракте, -
// это ошибка КОМПИЛЯЦИИ, а не тихий баг в проде. Обратное (схема шире
// рукописного типа) допустимо: фронт не обязан использовать все поля.
//
// Файл состоит только из типов - в бандл не попадает.

import type { components } from './generated/openapi';
import type { Program, Tournament, Game, User, Team } from '../types';

type Schemas = components['schemas'];

/**
 * Ключи Hand, отсутствующие в Spec. Должно быть never:
 * ненулевой результат печатается в ошибке компиляции списком полей-нарушителей.
 */
type ExtraKeys<Hand, Spec> = Exclude<keyof Hand & string, keyof Spec & string>;

type AssertNoDrift<T extends never> = T;

// При дрейфе тут появится ошибка вида:
// Type '"новое_поле"' does not satisfy the constraint 'never'.
export type ProgramDrift = AssertNoDrift<ExtraKeys<Program, Schemas['Program']>>;
export type TournamentDrift = AssertNoDrift<ExtraKeys<Tournament, Schemas['Tournament']>>;
export type GameDrift = AssertNoDrift<ExtraKeys<Game, Schemas['Game']>>;
export type UserDrift = AssertNoDrift<ExtraKeys<User, Schemas['User']>>;
export type TeamDrift = AssertNoDrift<ExtraKeys<Team, Schemas['Team']>>;

// Статусы программы должны совпадать со спекой по значениям.
type SpecProgramStatus = NonNullable<Schemas['Program']['status']>;
export type ProgramStatusDrift = AssertNoDrift<Exclude<Program['status'], SpecProgramStatus>>;
