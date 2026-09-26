// @vitest-environment happy-dom
import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { GameShowcase } from './GameShowcase';

// выплаты дилеммы как в tjudge-cli: 5/5, 0/10, 10/0, 1/1
describe('GameShowcase', () => {
  it('матрица и правила дилеммы на главной совпадают с игрой', () => {
    const { container } = render(<GameShowcase />);

    const cells = [...container.querySelectorAll('td span.font-mono')].map((el) => el.textContent);
    expect(cells).toEqual(['5, 5', '0, 10', '10, 0', '1, 1']);
    expect(screen.getByText(/оба получают по 5 очков/)).toBeTruthy();
    expect(screen.getByText(/предатель получает 10, жертва — 0/)).toBeTruthy();
    expect(screen.getByText(/оба получают по 1 очку/)).toBeTruthy();
  });
});
