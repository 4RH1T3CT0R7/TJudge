import type { HeadToHeadCell } from '../types';

// «Экология» Аксельрода: популяция стратегий, где доля каждой в следующем
// поколении растёт пропорционально её средним очкам против текущей смеси
// (репликаторная динамика). Очки пар - средние за матч из личных встреч.

/** Средние очки строки против колонки за матч; ids задают порядок. */
export function payoffMatrix(cells: HeadToHeadCell[], ids: string[]): number[][] {
  const avg = new Map<string, number>();
  for (const c of cells) {
    const n = c.wins + c.losses + c.draws;
    if (n > 0) avg.set(`${c.team_id}:${c.opponent_id}`, c.score_for / n);
  }
  return ids.map((row) => {
    const known = ids.flatMap((col) => (col !== row && avg.has(`${row}:${col}`) ? [avg.get(`${row}:${col}`)!] : []));
    // самоигры в турнире нет, а у пары без доигранных матчей данных нет:
    // там стратегия получает свои средние очки
    const mean = known.length > 0 ? known.reduce((s, v) => s + v, 0) / known.length : 0;
    return ids.map((col) => avg.get(`${row}:${col}`) ?? mean);
  });
}

/** Доли стратегий по поколениям: [0] - поровну, дальше generations шагов. */
export function replicate(payoff: number[][], generations: number): number[][] {
  const k = payoff.length;
  if (k === 0) return [];
  // приспособленность должна быть положительной: очки бывают нулевыми и
  // отрицательными (аукцион), поэтому вся матрица сдвигается вверх
  const min = Math.min(...payoff.flat());
  const shift = min <= 0 ? 1 - min : 0;
  let p = Array<number>(k).fill(1 / k);
  const history = [p];
  for (let g = 0; g < generations; g++) {
    const fitness = payoff.map((row) => row.reduce((s, v, j) => s + (v + shift) * p[j], 0));
    const mean = fitness.reduce((s, f, i) => s + f * p[i], 0);
    p = p.map((x, i) => (x * fitness[i]) / mean);
    history.push(p);
  }
  return history;
}
