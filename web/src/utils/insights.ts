import type { HeadToHeadCell, MatchRound, StrategyProfile } from '../types';

export interface Insight {
  /** Короткая подпись карточки. */
  label: string;
  text: string;
}

interface InsightInput {
  /** Места команд по итоговой таблице: id команды → название и место. */
  places: Map<string, { name: string; place: number }>;
  games: { id: string; name: string; display_name: string }[];
  /** Личные встречи по играм: id игры → клетки. */
  headToHead: Map<string, HeadToHeadCell[]>;
  rounds: MatchRound[];
  /** Свойства стратегий дилеммы; нет - дилеммы в турнире нет. */
  strategies?: StrategyProfile[];
}

const pct = (x: number) => `${Math.round(x * 100)}%`;
// обыграла ли команда строки соперника по сумме встреч пары
const beat = (c: HeadToHeadCell) => c.wins > c.losses;

// Автоинсайты по итогам турнира: кто обыгрывал лидера, главная сенсация,
// самая кооперативная стратегия, преимущество игрока 1, ничьи. Каждый - только
// если для него есть данные.
export function tournamentInsights({ places, games, headToHead, rounds, strategies }: InsightInput): Insight[] {
  const out: Insight[] = [];
  const gameName = (id: string) => games.find((g) => g.id === id)?.display_name ?? '';
  const pairs = games.flatMap((g) => (headToHead.get(g.id) ?? []).map((c) => ({ game: g.id, c })));

  const leader = [...places.entries()].find(([, p]) => p.place === 1);
  if (leader && pairs.length > 0) {
    const [id, { name }] = leader;
    // победители лидера, у каждого - игры, где он выиграл встречу
    const lost = new Map<string, string[]>();
    for (const { game, c } of pairs) {
      if (c.opponent_id !== id || !beat(c)) continue;
      lost.set(c.team_name, [...(lost.get(c.team_name) ?? []), `${gameName(game)} ${c.wins}–${c.losses}`]);
    }
    const list = [...lost].slice(0, 3).map(([team, g]) => `«${team}» (${g.join(', ')})`).join(', ');
    out.push({
      label: 'кто обыгрывал лидера',
      text:
        lost.size === 0
          ? `«${name}» не проиграла ни одной личной встречи`
          : `«${name}» уступила: ${list}${lost.size > 3 ? ` и ещё ${lost.size - 3}` : ''}`,
    });
  }

  // сенсация - победа команды, стоящей в таблице ниже всех относительно соперника
  let upset: { game: string; c: HeadToHeadCell; gap: number } | null = null;
  for (const { game, c } of pairs) {
    const a = places.get(c.team_id);
    const b = places.get(c.opponent_id);
    if (!a || !b || !beat(c)) continue;
    const gap = a.place - b.place;
    if (gap > 0 && (!upset || gap > upset.gap)) upset = { game, c, gap };
  }
  if (upset) {
    const { c, game } = upset;
    out.push({
      label: 'главная сенсация',
      text: `«${c.team_name}» (${places.get(c.team_id)!.place}-е место) обыграла «${c.opponent_name}» (${places.get(c.opponent_id)!.place}-е) в игре «${gameName(game)}», ${c.wins}–${c.losses}`,
    });
  }

  const coop = (strategies ?? []).filter((s) => s.cooperation !== null);
  if (coop.length > 1) {
    const sorted = [...coop].sort((a, b) => b.cooperation! - a.cooperation!);
    const [top, bottom] = [sorted[0], sorted.at(-1)!];
    out.push({
      label: 'самая кооперативная',
      text: `«${top.team_name}» сотрудничала в ${pct(top.cooperation!)} ходов дилеммы, меньше всех — «${bottom.team_name}», ${pct(bottom.cooperation!)}`,
    });
  }

  // матчи раундов по играм: победы игрока 1 и 2 и ничьи
  const byGame = new Map<string, { w1: number; w2: number; done: number }>();
  for (const r of rounds) {
    const t = byGame.get(r.game_type) ?? { w1: 0, w2: 0, done: 0 };
    byGame.set(r.game_type, { w1: t.w1 + r.wins1, w2: t.w2 + r.wins2, done: t.done + r.completed_count });
  }
  const title = (type: string) => games.find((g) => g.name === type)?.display_name ?? type;
  const seat = [...byGame.entries()]
    .filter(([, t]) => t.w1 + t.w2 >= 10)
    .map(([type, t]) => ({ type, share: t.w1 / (t.w1 + t.w2) }))
    .sort((a, b) => Math.abs(b.share - 0.5) - Math.abs(a.share - 0.5))[0];
  if (seat && Math.abs(seat.share - 0.5) >= 0.1) {
    out.push({
      label: 'очерёдность',
      text: `в игре «${title(seat.type)}» игрок ${seat.share > 0.5 ? 1 : 2} выигрывает ${pct(Math.max(seat.share, 1 - seat.share))} решённых матчей`,
    });
  }

  const draws = [...byGame.entries()]
    .filter(([, t]) => t.done >= 10)
    .map(([type, t]) => ({ type, share: (t.done - t.w1 - t.w2) / t.done }))
    .sort((a, b) => b.share - a.share)[0];
  if (draws && draws.share >= 0.3) {
    out.push({
      label: 'ничьи',
      text: `больше всего ничьих в игре «${title(draws.type)}»: ${pct(draws.share)} сыгранных матчей`,
    });
  }

  return out;
}
