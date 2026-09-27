// @vitest-environment happy-dom
import { act } from 'react';
import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import api from '../../api/client';
import { useConfirmStore } from '../../store/confirmStore';
import { ConfirmDialogHost } from '../ui/ConfirmDialog';
import { useGameAdminActions } from '../tournament/useGameAdminActions';
import { confirmDisqualify, count } from './confirmations';
import type { Game, MatchRound, Tournament } from '../../types';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  useConfirmStore.setState({ pending: null });
});

const round = (r: Partial<MatchRound>): MatchRound => ({
  round_number: 1, game_type: 'dilemma', total_matches: 0, completed_count: 0, pending_count: 0,
  running_count: 0, failed_count: 0, wins1: 0, wins2: 0, created_at: '', ...r,
});

it('склоняет счётчики по-русски', () => {
  const forms: [string, string, string] = ['матч', 'матча', 'матчей'];
  expect([1, 2, 5, 11, 21, 104].map((n) => count(n, forms))).toEqual([
    '1 матч', '2 матча', '5 матчей', '11 матчей', '21 матч', '104 матча',
  ]);
});

it('запуск раунда показывает числа dry-run и без подтверждения ничего не запускает', async () => {
  vi.spyOn(api, 'previewGameRound').mockResolvedValue({
    game_type: 'dilemma', pending: 0, participants: 5, matches_created: 20, matches_deleted: 12,
  });
  const run = vi.spyOn(api, 'runGameMatches').mockResolvedValue({ status: 'started', game_type: 'dilemma', enqueued: 20 });
  vi.spyOn(api, 'deactivateAllGames').mockResolvedValue();
  render(<ConfirmDialogHost />);

  const game = { id: 'g1', name: 'dilemma', display_name: 'Дилемма заключённого' } as Game;
  const client = new QueryClient();
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  const { result } = renderHook(() => useGameAdminActions({
    tournamentId: 't1', tournament: { id: 't1' } as Tournament, games: [game], setActionError: () => {},
  }), { wrapper });

  let pending!: Promise<void>;
  act(() => { pending = result.current.handleRunGameMatches('g1', 'dilemma', 'Дилемма заключённого'); });
  const dialog = await screen.findByRole('dialog');
  expect(dialog.textContent).toContain('удалит 12 матчей прошлого раунда');
  expect(dialog.textContent).toContain('создаст 20 матчей для 5 команд с готовой программой');
  expect(dialog.textContent).toContain('это последняя игра');
  // фокус на «Отмена»: Enter по привычке не перезапускает раунд
  expect(document.activeElement?.textContent).toBe('Отмена');

  fireEvent.click(screen.getByText('Отмена'));
  await act(() => pending);
  expect(run).not.toHaveBeenCalled();

  act(() => { pending = result.current.handleRunGameMatches('g1', 'dilemma', 'Дилемма заключённого'); });
  fireEvent.click(await screen.findByText('Перезапустить раунд'));
  await act(() => pending);
  expect(run).toHaveBeenCalledWith('t1', 'dilemma');
});

it('дисквалификация считает матчи команды и ждёт ввода её названия', async () => {
  const rounds = vi.spyOn(api, 'getMatchesByRounds').mockResolvedValue([
    round({ completed_count: 2, failed_count: 1, pending_count: 1, running_count: 1 }),
  ]);
  render(<ConfirmDialogHost />);

  let answer!: Promise<boolean>;
  act(() => { answer = confirmDisqualify('t1', { id: 'team1', name: 'Энтропия' }); });
  const dialog = await screen.findByRole('dialog');
  expect(rounds).toHaveBeenCalledWith('t1', undefined, 'team1');
  expect(dialog.textContent).toContain('удалит 3 сыгранных матча');
  expect(dialog.textContent).toContain('отменит 2 недоигранных матча');

  const input = screen.getByLabelText(/Чтобы подтвердить, введите/);
  const confirm = screen.getByText('Дисквалифицировать') as HTMLButtonElement;
  expect(document.activeElement).toBe(input);
  expect(confirm.disabled).toBe(true);

  // Enter с неверным текстом не подтверждает
  fireEvent.change(input, { target: { value: 'энтропия' } });
  fireEvent.submit(input.closest('form')!);
  expect(useConfirmStore.getState().pending).not.toBeNull();

  fireEvent.change(input, { target: { value: ' Энтропия ' } });
  expect(confirm.disabled).toBe(false);
  fireEvent.submit(input.closest('form')!);
  await waitFor(() => expect(useConfirmStore.getState().pending).toBeNull());
  await expect(answer).resolves.toBe(true);
});
