import { describe, expect, it } from 'vitest';
import { auctionTurns, cooperationShare, firstDefection, iterationsOf, runningTotal, sharePct } from './transcript';

describe('transcript', () => {
  it('счёт по ходу матча - нарастающий итог очков', () => {
    expect(runningTotal([5, 0, 10, 1])).toEqual([5, 5, 15, 16]);
    expect(runningTotal(undefined)).toEqual([]);
  });

  it('первое предательство и одновременное предательство', () => {
    expect(firstDefection({ moves: [[1, 1, 0], [1, 0, 0]] })).toEqual({ iteration: 1, sides: [2] });
    expect(firstDefection({ moves: [[1, 0], [1, 0]] })).toEqual({ iteration: 1, sides: [1, 2] });
    expect(firstDefection({ moves: [[1, 1], [1, 1]] })).toBeNull();
  });

  it('упавший матч: стороны разной длины', () => {
    const t = { moves: [[1, 0], [0]] };
    expect(iterationsOf(t)).toBe(2);
    expect(firstDefection(t)).toEqual({ iteration: 0, sides: [2] });
    expect(cooperationShare(t.moves[0])).toBe(0.5);
    expect(cooperationShare([])).toBeNull();
  });

  it('100% и 0% - только точные доли', () => {
    expect([1, 0.999, 0.62, 0.001, 0].map(sharePct)).toEqual(['100%', '>99%', '62%', '<1%', '0%']);
  });

  it('торги аукциона идут по очереди, первой ставит программа 1', () => {
    expect(auctionTurns({ moves: [[10, 0], [20]] })).toEqual([
      { side: 1, bid: 10 },
      { side: 2, bid: 20 },
      { side: 1, bid: 0 },
    ]);
  });
});
