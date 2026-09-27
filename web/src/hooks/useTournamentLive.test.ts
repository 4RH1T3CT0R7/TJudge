// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { createElement, type ReactNode } from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../api/client';
import type { MatchRound, Tournament } from '../types';
import { queryKeys } from '../api/queryKeys';
import { throttle, useTournamentLive, ROUND_POLL_INTERVAL, TOURNAMENT_STATUS_POLL_INTERVAL } from './useTournamentLive';
import { FALLBACK_POLL_INTERVAL } from './queries';

// связь есть, когда хук включён: сокет в тестах не открывается
vi.mock('./useWebSocket', () => ({
  useWebSocket: ({ enabled }: { enabled: boolean }) => ({ isConnected: enabled, isOnline: true, reconnect: () => {} }),
}));

describe('useTournamentLive без WS', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('вкладка, открытая до старта, включает поллинг, когда турнир пошёл', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const getTournament = vi
      .spyOn(api, 'getTournament')
      .mockResolvedValue({ id: 't1', status: 'pending' } as Tournament);
    const client = new QueryClient();
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);

    const { result } = renderHook(() => useTournamentLive({ tournamentId: 't1', enabled: false }), { wrapper });
    await waitFor(() => expect(getTournament).toHaveBeenCalledTimes(1));
    expect(result.current.pollInterval).toBe(false);

    getTournament.mockResolvedValue({ id: 't1', status: 'active' } as Tournament);
    await vi.advanceTimersByTimeAsync(TOURNAMENT_STATUS_POLL_INTERVAL);
    await waitFor(() => expect(result.current.pollInterval).toBe(FALLBACK_POLL_INTERVAL));
  });
});

describe('useTournamentLive с WS', () => {
  afterEach(() => vi.restoreAllMocks());

  it('пока идёт раунд, опрашивает и при живой связи: упавшие матчи событий не шлют', async () => {
    vi.spyOn(api, 'getTournament').mockResolvedValue({ id: 't1', status: 'active' } as Tournament);
    const client = new QueryClient();
    const rounds = (pending: number) => [{ game_type: 'tug', pending_count: pending, running_count: 0 } as MatchRound];
    client.setQueryData(queryKeys.matchesByRounds('t1'), rounds(3));
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(QueryClientProvider, { client }, children);

    const { result } = renderHook(() => useTournamentLive({ tournamentId: 't1' }), { wrapper });
    await waitFor(() => expect(result.current.pollInterval).toBe(ROUND_POLL_INTERVAL));

    act(() => {
      client.setQueryData(queryKeys.matchesByRounds('t1'), rounds(0));
    });
    await waitFor(() => expect(result.current.pollInterval).toBe(false));
  });
});

describe('throttle', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('первый вызов сразу, пачка внутри окна - одним хвостовым вызовом', () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    const call = throttle(fn, 5000);

    call();
    expect(fn).toHaveBeenCalledTimes(1);

    for (let i = 0; i < 100; i++) call();
    vi.advanceTimersByTime(4999);
    expect(fn).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1);
    expect(fn).toHaveBeenCalledTimes(2);

    // хвостовой вызов открывает новое окно: не чаще раза в 5с при потоке событий
    call();
    expect(fn).toHaveBeenCalledTimes(2);
    vi.advanceTimersByTime(5000);
    expect(fn).toHaveBeenCalledTimes(3);

    // без событий окно закрывается без лишних вызовов, следующее событие - сразу
    vi.advanceTimersByTime(20000);
    expect(fn).toHaveBeenCalledTimes(3);
    call();
    expect(fn).toHaveBeenCalledTimes(4);
  });

  it('cancel снимает отложенный хвостовой вызов', () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    const call = throttle(fn, 5000);

    call();
    call();
    call.cancel();
    vi.advanceTimersByTime(10000);
    expect(fn).toHaveBeenCalledTimes(1);
  });
});
