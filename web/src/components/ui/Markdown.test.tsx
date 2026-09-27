// @vitest-environment happy-dom
import { render } from '@testing-library/react';
import { expect, it } from 'vitest';
import { Markdown } from './Markdown';

// num у колонок: вправо и моноширинно выравниваются только колонки, где в теле одни числа
const numCols = (table: HTMLTableElement) =>
  [...table.rows[0].cells].map((_, col) => [...table.rows].every((row) => row.cells[col].classList.contains('num')));

it('числовые колонки таблиц правил выравниваются, текстовые и формулы - нет', () => {
  const { container } = render(
    <Markdown>
      {[
        '| Вы | Противник | Ваши очки | Очки противника |',
        '|---|---|---|---|',
        '| COOPERATE | COOPERATE | **5** | **5** |',
        '| COOPERATE | DEFECT | 0 | **10** |',
        '| DEFECT | DEFECT | 1 | 1 |',
        '',
        '| Вы | Противник | Ваши очки |',
        '|---|---|---|',
        '| 30 | 70 | 30+R = **32** |',
        '| 80 | -40 | 1.5 |',
      ].join('\n')}
    </Markdown>,
  );
  const [dilemma, travelers] = container.querySelectorAll('table');
  expect(numCols(dilemma)).toEqual([false, false, true, true]);
  expect(numCols(travelers)).toEqual([true, true, false]);
});
