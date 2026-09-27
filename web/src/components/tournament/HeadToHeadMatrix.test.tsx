// @vitest-environment happy-dom
import { afterEach, describe, it, expect } from 'vitest';
import { cleanup, render, screen, fireEvent } from '@testing-library/react';
import { HeadToHeadMatrix } from './HeadToHeadMatrix';
import type { HeadToHeadCell } from '../../types';

// каждая с каждой: 2 матча, строка выигрывает оба у команд с большим номером
function cells(n: number): HeadToHeadCell[] {
  const out: HeadToHeadCell[] = [];
  for (let a = 0; a < n; a++)
    for (let b = 0; b < n; b++) {
      if (a === b) continue;
      const won = a < b;
      out.push({
        team_id: `t${a}`,
        team_name: `К${a}`,
        opponent_id: `t${b}`,
        opponent_name: `К${b}`,
        wins: won ? 2 : 0,
        losses: won ? 0 : 2,
        draws: 0,
        score_for: won ? 1000 : 100,
        score_against: won ? 100 : 1000,
      });
    }
  return out;
}

describe('HeadToHeadMatrix', () => {
  afterEach(cleanup);

  it('средние очки - за матч, порядок строк - по месту', () => {
    render(<HeadToHeadMatrix cells={cells(3)} order={['t2', 't1', 't0']} />);
    fireEvent.click(screen.getByText('Средние очки'));
    const rows = screen.getAllByRole('row').slice(1);
    expect(rows.map((r) => r.querySelector('th')?.textContent)).toEqual(['К2', 'К1', 'К0']);
    // К2 проигрывает всем: 100 очков за 2 матча
    expect(rows[0].textContent).toContain('50');
  });

  it('больше 20 команд - пиксельная карта, пара выбирается стрелками', () => {
    render(<HeadToHeadMatrix cells={cells(21)} />);
    expect(screen.queryByRole('table')).toBeNull();
    const map = screen.getByRole('img');
    fireEvent.focus(map);
    // фокус ставит курсор на первую пару (К0, К1), вниз на две строки - К2 против К1
    fireEvent.keyDown(map, { key: 'ArrowDown' });
    fireEvent.keyDown(map, { key: 'ArrowDown' });
    expect(screen.getByText(/К2 против К1: 0–2/)).toBeTruthy();
  });
});
