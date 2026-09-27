// @vitest-environment happy-dom
import { afterEach, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { StatusLabel } from './StatusLabel';

afterEach(cleanup);

it('сопоставляет статус с глифом, подписью и тоном', () => {
  const cases = [
    ['match', 'failed', '✕ Ошибка', 'badge-red'],
    ['match', 'running', '◐ Идёт', 'badge-blue'],
    ['tournament', 'pending', '○ Регистрация', 'badge-yellow'],
    ['tournament', 'active', '◐ Активный', 'badge-blue'],
    ['program', 'compiling', '◔ Компилируется', 'badge-yellow'],
    ['program', 'ready', '● Готова', 'badge-green'],
    ['tournament', 'completed', '■ Завершён', 'badge-gray'],
  ] as const;
  for (const [entity, status, text, tone] of cases) {
    const { container } = render(<StatusLabel entity={entity} status={status} />);
    const badge = container.firstElementChild!;
    expect(badge.textContent).toBe(text);
    expect(badge.classList).toContain(tone);
    expect(badge.querySelector('[aria-hidden="true"]')?.textContent?.trim()).toBe(text[0]);
    cleanup();
  }
});

it('неизвестный статус выводится как есть серым и без глифа', () => {
  const { container } = render(<StatusLabel entity="match" status="paused" />);
  expect(container.textContent).toBe('paused');
  expect(container.firstElementChild!.classList).toContain('badge-gray');
});
