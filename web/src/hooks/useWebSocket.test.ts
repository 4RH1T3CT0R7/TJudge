// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { AxiosError, type AxiosAdapter, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios';
import { useWebSocket, reconnectDelay } from './useWebSocket';
import api from '../api/client';

// Управляемый WebSocket: тест сам решает, когда сокет открылся или упал.
class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  onopen: ((e: Event) => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  close = vi.fn();
  protocols: string[];

  constructor(_url: string, protocols: string[]) {
    this.protocols = protocols;
    FakeWebSocket.instances.push(this);
  }

  open() {
    this.onopen?.(new Event('open'));
  }

  drop(code: number) {
    this.onclose?.({ code } as CloseEvent);
  }
}

const last = () => FakeWebSocket.instances[FakeWebSocket.instances.length - 1];

// JWT с нужным exp (сек): подпись клиент не проверяет, ему нужен только payload
const jwt = (exp: number) => `h.${btoa(JSON.stringify({ exp }))}.s`;

const user = { id: 'u1', username: 'u', email: 'u@x.y', role: 'user', created_at: '', updated_at: '' };

describe('useWebSocket', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    FakeWebSocket.instances = [];
    vi.stubGlobal('WebSocket', FakeWebSocket);
    localStorage.setItem('access_token', 'token');
  });

  afterEach(() => {
    vi.useRealTimers();
    localStorage.clear();
  });

  it('backoff растёт до потолка 30с', () => {
    expect([1, 2, 3, 4, 5, 6, 7, 20].map(reconnectDelay)).toEqual([
      1000, 2000, 4000, 8000, 16000, 30000, 30000, 30000,
    ]);
  });

  it('переподключается без лимита попыток', () => {
    renderHook(() => useWebSocket({ tournamentId: 't1', enabled: true }));
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(FakeWebSocket.instances).toHaveLength(1);

    for (let attempt = 1; attempt <= 10; attempt++) {
      act(() => {
        last().drop(1006);
        vi.advanceTimersByTime(reconnectDelay(attempt));
      });
    }
    expect(FakeWebSocket.instances).toHaveLength(11);
  });

  it('onclose заменённого сокета не трогает текущий', () => {
    const { result } = renderHook(() => useWebSocket({ tournamentId: 't1', enabled: true }));
    act(() => {
      vi.advanceTimersByTime(100);
    });
    const stale = last();
    act(() => stale.open());

    act(() => result.current.reconnect());
    const current = last();
    expect(current).not.toBe(stale);
    act(() => current.open());
    expect(result.current.isConnected).toBe(true);

    // close старого сокета доходит уже после открытия нового
    act(() => {
      stale.drop(1005);
      vi.advanceTimersByTime(60000);
    });
    expect(result.current.isConnected).toBe(true);
    expect(FakeWebSocket.instances).toHaveLength(2);
  });

  it('после истечения токена reconnect ждёт общий с REST refresh и берёт новый токен', async () => {
    const now = Math.floor(Date.now() / 1000);
    const stale = jwt(now + 60);
    const fresh = jwt(now + 3600);
    api.setAccessToken(stale);
    localStorage.setItem('refresh_token', 'r1');

    let refreshCalls = 0;
    let releaseRefresh!: () => void;
    const refreshGate = new Promise<void>((resolve) => (releaseRefresh = resolve));
    const respond = (config: InternalAxiosRequestConfig, status: number, data: unknown): AxiosResponse => ({
      data: { data }, status, statusText: '', headers: { 'content-type': 'application/json' }, config,
    });
    const adapter: AxiosAdapter = async (config) => {
      if (config.url === '/auth/refresh') {
        refreshCalls++;
        await refreshGate;
        return respond(config, 200, { access_token: fresh, refresh_token: 'r2', user });
      }
      if (config.headers.Authorization === `Bearer ${fresh}`) return respond(config, 200, user);
      throw new AxiosError('unauthorized', undefined, config, null, respond(config, 401, null));
    };
    (api as unknown as { client: { defaults: { adapter: AxiosAdapter } } }).client.defaults.adapter = adapter;

    renderHook(() => useWebSocket({ tournamentId: 't1', enabled: true }));
    act(() => {
      vi.advanceTimersByTime(100);
    });
    act(() => last().open());

    // Токен истёк, связь оборвалась, одновременно REST-запрос получил 401
    vi.setSystemTime(Date.now() + 120_000);
    act(() => last().drop(1006));
    const me = api.getMe();
    await act(() => vi.advanceTimersByTimeAsync(reconnectDelay(1)));
    // Со старым токеном сокет не открывается, пока идёт refresh
    expect(FakeWebSocket.instances).toHaveLength(1);

    releaseRefresh();
    await act(() => vi.advanceTimersByTimeAsync(0));
    await expect(me).resolves.toEqual(user);
    expect(refreshCalls).toBe(1);
    expect(FakeWebSocket.instances).toHaveLength(2);
    expect(last().protocols).toEqual([`access_token.${fresh}`]);
  });
});
