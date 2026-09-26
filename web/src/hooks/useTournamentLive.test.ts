// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { createElement, type ReactNode } from 'react';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import api from '../api/client';
import type { Tournament } from '../types';
import { throttle, useTournamentLive, TOURNAMENT_STATUS_POLL_INTERVAL } from './useTournamentLive';
import { FALLBACK_POLL_INTERVAL } from './queries';

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
