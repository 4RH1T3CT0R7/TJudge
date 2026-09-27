import { Link } from 'react-router-dom';
import { YouMark } from '../ui/YouMark';
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
  const filled = Math.round(value * 5);
  return (
    <>
      <span aria-hidden="true" className="text-primary-400">{'█'.repeat(filled)}</span>
      <span aria-hidden="true" className="text-gray-700">{'░'.repeat(5 - filled)}</span>
      <span className="ml-2 inline-block w-[4ch] text-right text-gray-200">{Math.round(value * 100)}%</span>
    </>
  );
}

// Паспорт стратегий дилеммы: свойства каждой команды по транскриптам её матчей.
// Число матчей ведёт к списку этих матчей, откуда можно открыть ходы каждого.
export function StrategyPassport({ profiles, myTeamId }: { profiles: StrategyProfile[]; myTeamId?: string }) {
  return (
    <div>
      <div className="relative overflow-x-auto">
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
                  <td className="py-2 pr-4 text-gray-200">
                    {p.team_name}
                    {mine && <YouMark />}
                  </td>
                  {TRAITS.map((t) => (
                    <td key={t.key} className="py-2 pr-4 font-mono tabular-nums">
                      <Share value={p[t.key]} />
                    </td>
                  ))}
                  <td className="py-2 text-right font-mono tabular-nums">
                    <Link
                      to={{ search: `?tab=matches&team=${p.team_id}` }}
                      aria-label={`Матчи команды ${p.team_name}: ${p.matches}`}
                      className="text-primary-400 underline hover:text-primary-300"
                    >
                      {p.matches}
                    </Link>
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
