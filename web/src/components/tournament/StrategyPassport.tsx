import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { YouMark } from '../ui/YouMark';
import { sharePct } from '../../utils/transcript';
import type { StrategyProfile } from '../../types';

type Trait = 'cooperation' | 'niceness' | 'retaliation' | 'forgiveness' | 'provocability';

// свойства стратегии по Аксельроду и что они значат простыми словами
const TRAITS: { key: Trait; label: string; hint: string }[] = [
  { key: 'cooperation', label: 'сотрудничество', hint: 'какую долю всех своих ходов команда сотрудничала' },
  {
    key: 'niceness',
    label: 'добрая',
    hint: 'не предаёт первой: доля матчей, где команда не предала раньше соперника (предать в один ход с ним — тоже первой)',
  },
  {
    key: 'retaliation',
    label: 'мстительная',
    hint: 'отвечает ударом на удар: как часто после предательства соперника следующий ход команды — тоже предательство',
  },
  {
    key: 'forgiveness',
    label: 'прощающая',
    hint: 'не держит зла: как часто команда снова сотрудничает, когда соперник после предательства вернулся к сотрудничеству',
  },
  {
    key: 'provocability',
    label: 'провоцируемая',
    hint: 'не терпит беспричинных ударов: как часто команда отвечает предательством, если соперник предал, хотя она сама перед этим сотрудничала',
  },
];

// доля шкалой из пяти клеток и процентом: «███░░ 62%»
function Share({ value }: { value: number | null }) {
  if (value === null) {
    return (
      <span className="text-gray-500" title="ситуации не было">
        —
      </span>
    );
  }
  // полная шкала и пустая - только у точных 100% и 0%
  const filled = value === 0 ? 0 : value === 1 ? 5 : Math.min(4, Math.max(1, Math.round(value * 5)));
  return (
    <>
      <span aria-hidden="true" className="text-primary-400">{'█'.repeat(filled)}</span>
      <span aria-hidden="true" className="text-gray-700">{'░'.repeat(5 - filled)}</span>
      <span className="ml-2 inline-block w-[4ch] text-right text-gray-200">{sharePct(value)}</span>
    </>
  );
}

// число матчей команды - ссылка на их список, откуда можно открыть ходы каждого
function MatchesLink({ p, children }: { p: StrategyProfile; children: ReactNode }) {
  return (
    <Link
      to={{ search: `?tab=matches&team=${p.team_id}` }}
      state={{ focus: 'matches' }}
      aria-label={`Матчи команды ${p.team_name}: ${p.matches}`}
      className="text-primary-400 underline hover:text-primary-300"
    >
      {children}
    </Link>
  );
}

// Паспорт стратегий дилеммы: свойства каждой команды по транскриптам её матчей.
// Пять свойств в строку таблицы помещаются только на широком экране: колонка
// рядом с программой на десктопе уже таблицы, поэтому ниже xl - карточка на команду.
export function StrategyPassport({ profiles, myTeamId }: { profiles: StrategyProfile[]; myTeamId?: string }) {
  return (
    <div>
      <ul className="space-y-3 xl:hidden">
        {profiles.map((p) => {
          const mine = p.team_id === myTeamId;
          return (
            <li
              key={p.team_id}
              aria-current={mine ? 'true' : undefined}
              className={`rounded border border-gray-800 p-3 ${mine ? 'row-mine' : ''}`}
            >
              <p className="flex items-baseline justify-between gap-3 text-sm">
                <span className="min-w-0 break-words text-gray-200">
                  {mine && <><YouMark className="mr-1 text-xs" />{' '}</>}
                  {p.team_name}
                </span>
                <span className="shrink-0 font-mono text-gray-400">
                  <MatchesLink p={p}>матчей: {p.matches}</MatchesLink>
                </span>
              </p>
              <dl className="mt-2 space-y-1 font-mono text-sm tabular-nums">
                {TRAITS.map((t) => (
                  <div key={t.key} className="flex items-center justify-between gap-3">
                    <dt className="text-gray-400">{t.label}</dt>
                    <dd>
                      <Share value={p[t.key]} />
                    </dd>
                  </div>
                ))}
              </dl>
            </li>
          );
        })}
      </ul>
      <div tabIndex={0} role="region" aria-label="Таблица паспорта стратегий" className="relative hidden overflow-x-auto xl:block">
        <table className="w-full whitespace-nowrap text-sm">
          <thead>
            <tr className="border-b border-gray-700 text-left text-gray-400">
              <th className="pb-2 pr-4">Команда</th>
              {TRAITS.map((t) => (
                <th key={t.key} className="pb-2 pr-4 font-mono font-normal" title={t.hint}>
                  {t.label}
                </th>
              ))}
              <th className="pb-2 text-right">Матчей</th>
            </tr>
          </thead>
          <tbody>
            {profiles.map((p) => {
              const mine = p.team_id === myTeamId;
              return (
                <tr key={p.team_id} aria-current={mine ? 'true' : undefined} className={`border-b border-gray-800 ${mine ? 'row-mine' : ''}`}>
                  <td className="whitespace-normal py-2 pr-4 text-gray-200">
                    {p.team_name}
                    {mine && <YouMark />}
                  </td>
                  {TRAITS.map((t) => (
                    <td key={t.key} className="py-2 pr-4 font-mono tabular-nums">
                      <Share value={p[t.key]} />
                    </td>
                  ))}
                  <td className="py-2 text-right font-mono tabular-nums">
                    <MatchesLink p={p}>{p.matches}</MatchesLink>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <dl className="mt-4 space-y-1 text-sm">
        {TRAITS.map((t) => (
          <div key={t.key}>
            <dt className="inline font-mono text-gray-200">{t.label}</dt>
            <dd className="inline text-gray-400"> — {t.hint}</dd>
          </div>
        ))}
      </dl>
      <p className="mt-3 font-mono text-xs text-gray-500">
        <span aria-hidden="true">{'// '}</span>
        «—» — ситуации не было: например, соперники ни разу не предавали и мстить было не за что. У Аксельрода чаще
        выигрывали добрые, провоцируемые и прощающие стратегии, как Tit-for-Tat
      </p>
    </div>
  );
}
