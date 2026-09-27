import type { HeadToHeadCell } from '../types';

// «Экология» Аксельрода: популяция стратегий, где доля каждой в следующем
// поколении растёт пропорционально её средним очкам против текущей смеси
// (репликаторная динамика). Очки пар - средние за матч из личных встреч.

/** Средние очки строки против колонки за матч; ids задают порядок. null - против себя или пара без доигранных матчей. */
export function payoffMatrix(cells: HeadToHeadCell[], ids: string[]): (number | null)[][] {
  const avg = new Map<string, number>();
  for (const c of cells) {
    const n = c.wins + c.losses + c.draws;
    if (n > 0) avg.set(`${c.team_id}:${c.opponent_id}`, c.score_for / n);
  }
  return ids.map((row) => ids.map((col) => avg.get(`${row}:${col}`) ?? null));
}

/** Доли стратегий по поколениям: [0] - поровну, дальше generations шагов. */
export function replicate(payoff: (number | null)[][], generations: number): number[][] {
  const k = payoff.length;
  if (k === 0) return [];
  // приспособленность должна быть положительной: очки бывают нулевыми и
  // отрицательными (аукцион), поэтому вся матрица сдвигается вверх
  const min = Math.min(...payoff.flat().filter((v) => v !== null));
  const shift = min <= 0 ? 1 - min : 0;
  let p = Array<number>(k).fill(1 / k);
  const history = [p];
  for (let g = 0; g < generations; g++) {
    // самоигры в турнире нет, и выдумывать её нельзя: у эксплуататора средние
    // против всех завышены. Поэтому стратегия встречает только остальную смесь
    // (и тех, с кем есть данные), а доли соперников нормируются на их сумму
    const fitness = payoff.map((row) => {
      let sum = 0;
      let weight = 0;
      row.forEach((v, j) => {
        if (v === null) return;
        sum += (v + shift) * p[j];
        weight += p[j];
      });
      // встречать некого: осталась одна стратегия или по ней нет данных
      return weight > 0 ? sum / weight : 1;
    });
    const mean = fitness.reduce((s, f, i) => s + f * p[i], 0);
    p = p.map((x, i) => (x * fitness[i]) / mean);
    history.push(p);
  }
  return history;
}
