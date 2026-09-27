// @vitest-environment happy-dom
import { afterEach, expect, it, vi } from 'vitest';
import { render } from '@testing-library/react';
import { TerminalOutput } from './TerminalOutput';

it('подсвечивает ошибки и предупреждения компилятора', () => {
  const log = [
    'main.c:3:5: warning: unused variable',
    'main.c:7:1: error: expected ;',
    './main.go:5:2: undefined: x',
    'ZeroDivisionError: division by zero',
    '1 warning and 1 error generated.',
    'Traceback (most recent call last):',
  ].join('\n');
  const { container } = render(<TerminalOutput text={log} label="stderr:" />);
  const tones = [...container.querySelectorAll('pre > span')].map((s) => s.className);
  expect(tones).toEqual(['text-amber-300', 'text-red-300', 'text-red-300', 'text-red-300', 'text-amber-300', '']);
  expect(container.querySelector('pre')!.textContent).toBe(log + '\n');
});

afterEach(() => vi.restoreAllMocks());

it('фокусируется с клавиатуры, только когда вывод прокручивается', () => {
  const { container, rerender } = render(<TerminalOutput text="ok" label="stderr:" />);
  expect(container.querySelector('pre')!.hasAttribute('tabindex')).toBe(false);

  vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(500);
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(100);
  rerender(<TerminalOutput text={'длинный\nвывод'} label="stderr:" />);
  const pre = container.querySelector('pre')!;
  expect(pre.tabIndex).toBe(0);
  expect(pre.getAttribute('role')).toBe('group');
  expect(pre.getAttribute('aria-label')).toBe('stderr:');
});
