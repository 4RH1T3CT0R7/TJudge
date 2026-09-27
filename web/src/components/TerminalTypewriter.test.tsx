// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import { TerminalTypewriter } from './TerminalTypewriter';

describe('TerminalTypewriter', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('печатает фразы один круг и останавливается на последней', () => {
    vi.useFakeTimers();
    render(<TerminalTypewriter />);
    for (let i = 0; i < 200; i++) {
      act(() => {
        vi.advanceTimersByTime(500);
      });
    }
    expect(screen.getByText('Оптимальность по Парето')).toBeTruthy();
    expect(vi.getTimerCount()).toBe(0);
  });
});
