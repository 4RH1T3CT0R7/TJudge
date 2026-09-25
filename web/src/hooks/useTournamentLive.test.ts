// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { throttle } from './useTournamentLive';

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
