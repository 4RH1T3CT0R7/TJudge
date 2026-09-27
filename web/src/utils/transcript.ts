import type { MatchTranscript } from '../types';
import type { Side } from './explainError';

// ходы дилеммы в транскрипте
export const COOPERATE = 1;
export const DEFECT = 0;

/** Счёт после каждой итерации: нарастающий итог очков. */
export function runningTotal(points: number[] = []): number[] {
  let sum = 0;
  return points.map((p) => (sum += p));
}

/** Итераций в транскрипте: у упавшего матча стороны бывают разной длины. */
export const iterationsOf = (t: MatchTranscript) => Math.max(t.moves[0]?.length ?? 0, t.moves[1]?.length ?? 0);

export interface FirstDefection {
  /** Итерация с нуля. */
  iteration: number;
  /** Кто предал на ней: обе стороны, если одновременно. */
  sides: Side[];
}

// Первое предательство в дилемме; null - обе стороны сотрудничали до конца.
export function firstDefection(t: MatchTranscript): FirstDefection | null {
  const [a = [], b = []] = t.moves;
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    const sides = ([1, 2] as const).filter((s) => (s === 1 ? a : b)[i] === DEFECT);
    if (sides.length > 0) return { iteration: i, sides };
  }
  return null;
}

/** Одиночный ход: на два хода в обе стороны только ходы другого вида. */
export const isLoneMove = (moves: number[], i: number) => [-2, -1, 1, 2].every((d) => moves[i + d] !== moves[i]);

/** Доля в процентах: 0% и 100% только для точных значений, рядом с ними - «<1%» и «>99%». */
export function sharePct(x: number): string {
  const r = Math.round(x * 100);
  if (r === 100 && x < 1) return '>99%';
  if (r === 0 && x > 0) return '<1%';
  return `${r}%`;
}

/** Доля сотрудничества стороны в дилемме; null - ходов нет. */
export function cooperationShare(moves: number[] = []): number | null {
  if (moves.length === 0) return null;
  return moves.filter((m) => m === COOPERATE).length / moves.length;
}

/** Средний ход стороны в числовых играх; null - ходов нет. */
export function averageMove(moves: number[] = []): number | null {
  if (moves.length === 0) return null;
  return moves.reduce((s, m) => s + m, 0) / moves.length;
}

// Торги аукциона по порядку: программа 1 ставит первой в каждом круге, 0 - пас.
export function auctionTurns(t: MatchTranscript): { side: Side; bid: number }[] {
  const [a = [], b = []] = t.moves;
  const turns: { side: Side; bid: number }[] = [];
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    if (i < a.length) turns.push({ side: 1, bid: a[i] });
    if (i < b.length) turns.push({ side: 2, bid: b[i] });
  }
  return turns;
}
