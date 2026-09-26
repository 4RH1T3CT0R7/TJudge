// @vitest-environment happy-dom
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MatchesTab } from './MatchesTab';
import api from '../../api/client';
import type { MatchRound } from '../../types';

const round: MatchRound = {
  round_number: 1,
  game_type: 'dilemma',
  total_matches: 120,
  completed_count: 100,
  pending_count: 20,
  running_count: 0,
  failed_count: 0,
  wins1: 60,
  wins2: 30,
  created_at: '2026-01-01T00:00:00Z',
};

describe('MatchesTab', () => {
  it('матчи раунда грузятся страницами только после раскрытия', async () => {
    const getRoundMatches = vi.spyOn(api, 'getRoundMatches').mockResolvedValue([]);
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MatchesTab
          tournamentId="t1"
          rounds={[round]}
          onRefresh={() => {}}
          isRefreshing={false}
          isAdmin={false}
          pollInterval={false}
        />
      </QueryClientProvider>
    );

    expect(getRoundMatches).not.toHaveBeenCalled();
    // ничьи - остаток завершённых после побед
    fireEvent.click(screen.getByText('Раунд 1'));
    expect(screen.getByText('10')).toBeTruthy();

    await waitFor(() => expect(getRoundMatches).toHaveBeenCalledWith('t1', 1, 'dilemma', 50, 0, expect.any(AbortSignal)));
    expect(screen.getByText('Страница 1 из 3')).toBeTruthy();

    fireEvent.click(screen.getByText('Вперёд'));
    await waitFor(() => expect(getRoundMatches).toHaveBeenCalledWith('t1', 1, 'dilemma', 50, 50, expect.any(AbortSignal)));
  });

  it('раунд с отменёнными матчами закрыт на 100%', () => {
    // 20 отменённых после очистки очереди ни в один счётчик не входят
    const cancelledRound = { ...round, pending_count: 0 };
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MatchesTab
          tournamentId="t1"
          rounds={[cancelledRound]}
          onRefresh={() => {}}
          isRefreshing={false}
          isAdmin={false}
          pollInterval={false}
        />
      </QueryClientProvider>
    );

    expect(screen.getByText('100%')).toBeTruthy();
  });
});
