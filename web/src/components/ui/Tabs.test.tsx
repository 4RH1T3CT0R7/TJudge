// @vitest-environment happy-dom
import { afterEach, expect, it } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { Tabs } from './Tabs';
import { useTabParam } from '../../hooks/useTabParam';

afterEach(cleanup);

const IDS = ['info', 'matches', 'teams'] as const;

function Page() {
  const [tab, setTab] = useTabParam(IDS, 'info');
  const { search } = useLocation();
  return (
    <>
      <output>{search}</output>
      <Tabs
        label="Разделы"
        active={tab}
        onChange={setTab}
        items={[
          { id: 'info', label: 'Информация' },
          { id: 'matches', label: 'Матчи', count: 414 },
          { id: 'teams', label: 'Команды' },
        ]}
      >
        панель {tab}
      </Tabs>
    </>
  );
}

const renderAt = (url: string) =>
  render(
    <MemoryRouter initialEntries={[url]}>
      <Page />
    </MemoryRouter>,
  );

it('берёт вкладку из ?tab= и пишет выбор обратно в URL', () => {
  renderAt('/t?tab=matches&x=1');
  const matches = screen.getByRole('tab', { name: /Матчи/ });
  expect(matches.getAttribute('aria-selected')).toBe('true');
  expect(matches.textContent).toBe('> Матчи [414]');
  expect(screen.getByRole('tabpanel').textContent).toBe('панель matches');

  fireEvent.click(screen.getByRole('tab', { name: /Команды/ }));
  expect(screen.getByRole('status').textContent).toBe('?tab=teams&x=1');

  // вкладка по умолчанию из URL убирается, чужие параметры остаются
  fireEvent.click(screen.getByRole('tab', { name: /Информация/ }));
  expect(screen.getByRole('status').textContent).toBe('?x=1');
});

it('неизвестное значение ?tab= открывает вкладку по умолчанию', () => {
  renderAt('/t?tab=bogus');
  expect(screen.getByRole('tab', { name: /Информация/ }).getAttribute('aria-selected')).toBe('true');
  expect(screen.getByRole('tabpanel').getAttribute('aria-labelledby')).toBe(
    screen.getByRole('tab', { name: /Информация/ }).id,
  );
});
