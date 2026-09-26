// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AutoRoundCountdown } from './AutoRoundCountdown';
import type { TournamentGameWithDetails } from '../../types';

const status = (over: Partial<TournamentGameWithDetails>): TournamentGameWithDetails => ({
  tournament_id: 't1', game_id: 'g1', game_name: 'dilemma', game_display_name: 'Дилемма',
  is_active: true, round_completed: false, auto_round_enabled: true, auto_round_interval_seconds: 60,
  ...over,
});

function text(s: TournamentGameWithDetails, tournamentActive = true) {
  const { container } = render(
    <QueryClientProvider client={new QueryClient()}>
      <AutoRoundCountdown status={s} tournamentActive={tournamentActive} />
    </QueryClientProvider>
  );
  return container.textContent;
}

describe('AutoRoundCountdown', () => {
  afterEach(cleanup);

  it('после интервала показывает причину, а не «вот-вот»', () => {
    const long = new Date(Date.now() - 3600_000).toISOString();
    expect(text(status({ auto_round_last_run_at: long, auto_round_wait: 'new_programs' }))).toBe('авто: ждёт новую версию программы');
    expect(text(status({ auto_round_last_run_at: long }))).toBe('раунд стартует');
  });

  it('до конца интервала - отсчёт, у неактивного турнира - ничего', () => {
    const now = new Date().toISOString();
    expect(text(status({ auto_round_last_run_at: now, auto_round_wait: 'interval' }))).toMatch(/^следующий раунд через (00:59|01:00)$/);
    expect(text(status({ auto_round_last_run_at: now }), false)).toBe('');
  });
});
