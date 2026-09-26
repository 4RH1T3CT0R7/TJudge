// @vitest-environment happy-dom
import { describe, it, expect, vi } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useOnRoundFinished } from './useOnRoundFinished';

describe('useOnRoundFinished', () => {
  it('срабатывает раз за раунд и не срабатывает на уже сыгранном', () => {
    const onFinish = vi.fn();
    const { rerender } = renderHook(({ running }) => useOnRoundFinished(running, onFinish), {
      initialProps: { running: false },
    });
    rerender({ running: false });
    expect(onFinish).not.toHaveBeenCalled();

    rerender({ running: true });
    rerender({ running: true });
    rerender({ running: false });
    rerender({ running: false });
    expect(onFinish).toHaveBeenCalledTimes(1);

    rerender({ running: true });
    rerender({ running: false });
    expect(onFinish).toHaveBeenCalledTimes(2);
  });
});
