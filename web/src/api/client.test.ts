// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import axios, { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios';
import { api } from './client';
import { useToastStore } from '../store/toastStore';

type Handler = (config: InternalAxiosRequestConfig) => { status: number; data?: unknown } | 'network';

const calls: { method: string; url: string; auth: string }[] = [];

// Подменяет транспорт axios: каждый запрос отвечает handler без сети.
function serve(handler: Handler) {
  const adapter: AxiosAdapter = async (config) => {
    calls.push({
      method: (config.method ?? 'get').toUpperCase(),
      url: config.url ?? '',
      auth: String(config.headers?.Authorization ?? ''),
    });
    const res = handler(config);
    if (res === 'network') {
      throw new AxiosError('Network Error', 'ERR_NETWORK', config);
    }
    const response = {
      data: { data: res.data },
      status: res.status,
      statusText: '',
      headers: { 'content-type': 'application/json' },
      config,
    };
    if (res.status >= 400) {
      throw new AxiosError('fail', undefined, config, null, response);
    }
    return response;
  };
  (api as unknown as { client: { defaults: { adapter: AxiosAdapter } } }).client.defaults.adapter = adapter;
}

const user = { id: 'u1', username: 'u', email: 'u@x.y', role: 'user', created_at: '', updated_at: '' };

function login(access: string, refresh: string) {
  api.setAccessToken(access);
  localStorage.setItem('refresh_token', refresh);
}

describe('ApiClient refresh', () => {
  const onAuthFailure = vi.fn();

  beforeEach(() => {
    calls.length = 0;
    localStorage.clear();
    onAuthFailure.mockReset();
    vi.restoreAllMocks();
    api.setOnAuthFailure(onAuthFailure);
  });

  it('401 на /auth/me обновляет токен и повторяет запрос', async () => {
    login('a1', 'r1');
    serve((c) => {
      if (c.url === '/auth/refresh') return { status: 200, data: { access_token: 'a2', refresh_token: 'r2', user } };
      return c.headers.Authorization === 'Bearer a2' ? { status: 200, data: user } : { status: 401 };
    });

    await expect(api.getMe()).resolves.toEqual(user);
    expect(calls.map((c) => c.url)).toEqual(['/auth/me', '/auth/refresh', '/auth/me']);
    expect(localStorage.getItem('refresh_token')).toBe('r2');
  });

  it('сетевая ошибка на refresh не стирает токены', async () => {
    login('a1', 'r1');
    serve((c) => (c.url === '/auth/refresh' ? 'network' : { status: 401 }));

    const err = await api.getMe().catch((e: unknown) => e);
    expect((err as AxiosError).response).toBeUndefined();
    expect(localStorage.getItem('access_token')).toBe('a1');
    expect(localStorage.getItem('refresh_token')).toBe('r1');
    expect(onAuthFailure).not.toHaveBeenCalled();
  });

  it('отказ refresh с 401 стирает токены', async () => {
    login('a1', 'r1');
    serve(() => ({ status: 401 }));

    const err = await api.getMe().catch((e: unknown) => e);
    expect((err as AxiosError).response?.status).toBe(401);
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(localStorage.getItem('refresh_token')).toBeNull();
    expect(onAuthFailure).toHaveBeenCalledOnce();
  });

  it('сбой браузера при refresh не стирает токены, отсутствие refresh-токена стирает', async () => {
    login('a1', 'r1');
    serve(() => ({ status: 401 }));
    const refresh = vi.spyOn(api, 'refreshToken').mockRejectedValue(new DOMException('aborted', 'AbortError'));

    await expect(api.getMe()).rejects.toBeInstanceOf(DOMException);
    expect(localStorage.getItem('access_token')).toBe('a1');
    expect(onAuthFailure).not.toHaveBeenCalled();

    refresh.mockRestore();
    localStorage.removeItem('refresh_token');
    await expect(api.getMe()).rejects.toBeInstanceOf(AxiosError);
    expect(localStorage.getItem('access_token')).toBeNull();
    expect(onAuthFailure).toHaveBeenCalledOnce();
  });

  it('проигравшая гонку вкладка берёт токены соседки, а не стирает их', async () => {
    login('a1', 'r1');
    serve((c) => {
      if (c.url === '/auth/refresh') {
        // r1 уже использовала соседняя вкладка и успела сохранить новую пару
        localStorage.setItem('access_token', 'a2');
        localStorage.setItem('refresh_token', 'r2');
        return { status: 401 };
      }
      return c.headers.Authorization === 'Bearer a2' ? { status: 200, data: user } : { status: 401 };
    });

    await expect(api.getMe()).resolves.toEqual(user);
    expect(localStorage.getItem('refresh_token')).toBe('r2');
    expect(onAuthFailure).not.toHaveBeenCalled();
  });

  it('истёкший токен соседней вкладки не заменяет refresh', async () => {
    login('a0', 'r1');
    const expired = `h.${btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) - 60 }))}.s`;
    localStorage.setItem('access_token', expired);
    serve((c) => {
      if (c.url === '/auth/refresh') return { status: 200, data: { access_token: 'a2', refresh_token: 'r2', user } };
      return c.headers.Authorization === 'Bearer a2' ? { status: 200, data: user } : { status: 401 };
    });

    await expect(api.getMe()).resolves.toEqual(user);
    expect(calls.map((c) => c.url)).toEqual(['/auth/me', '/auth/refresh', '/auth/me']);
  });
});

it('отменённый запрос не ретраится и не показывает тост', async () => {
  calls.length = 0;
  useToastStore.setState({ toasts: [] });
  serve(() => ({ status: 200, data: [] }));
  const controller = new AbortController();
  controller.abort();

  const err = await api.getMatchesByRounds('t1', controller.signal).catch((e: unknown) => e);
  expect(axios.isCancel(err)).toBe(true);
  expect(calls).toHaveLength(0);
  expect(useToastStore.getState().toasts).toHaveLength(0);
});
