// @vitest-environment happy-dom
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useWebSocket, reconnectDelay } from './useWebSocket';

// Node 25+ держит свой глобальный localStorage (без --localstorage-file он
// undefined), и тот закрывает хранилище happy-dom.
vi.hoisted(() => {
  if (typeof localStorage === 'undefined') vi.stubGlobal('localStorage', new Storage());
});

// Управляемый WebSocket: тест сам решает, когда сокет открылся или упал.
class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  onopen: ((e: Event) => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  close = vi.fn();

  constructor() {
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
});
