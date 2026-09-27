// @vitest-environment happy-dom
import { expect, it } from 'vitest';
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
