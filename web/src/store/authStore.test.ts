// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { AxiosError, AxiosHeaders } from 'axios';
import { useAuthStore } from './authStore';
import api from '../api/client';

// Node 25+ держит свой глобальный localStorage (без --localstorage-file он
// undefined), и тот закрывает хранилище happy-dom.
vi.hoisted(() => {
  if (typeof localStorage === 'undefined') vi.stubGlobal('localStorage', new Storage());
});

const config = { headers: new AxiosHeaders() };
const httpError = (status?: number) =>
  new AxiosError('fail', undefined, config, null, status
    ? { status, statusText: '', headers: {}, config, data: {} }
    : undefined);

describe('authStore', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
    useAuthStore.setState({ user: null, isAuthenticated: false, isInitialized: false, isLoading: false });
    api.setAccessToken('a1');
    localStorage.setItem('refresh_token', 'r1');
  });

  it('initialize не стирает токены при сетевой ошибке и 5xx', async () => {
    for (const err of [httpError(), httpError(503)]) {
      useAuthStore.setState({ isInitialized: false });
      vi.spyOn(api, 'getMe').mockRejectedValueOnce(err);
      await useAuthStore.getState().initialize();
      expect(useAuthStore.getState().isInitialized).toBe(true);
      expect(localStorage.getItem('refresh_token')).toBe('r1');
    }
  });

  it('initialize сбрасывает сессию на 401', async () => {
    vi.spyOn(api, 'getMe').mockRejectedValueOnce(httpError(401));
    await useAuthStore.getState().initialize();
    expect(useAuthStore.getState()).toMatchObject({ isAuthenticated: false, isInitialized: true });
    expect(localStorage.getItem('refresh_token')).toBeNull();
  });
});
