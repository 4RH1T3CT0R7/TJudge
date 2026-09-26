// @vitest-environment happy-dom
import { describe, it, expect, vi } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useOnRoundFinished } from './useOnRoundFinished';

describe('useOnRoundFinished', () => {
  it('срабатывает раз за раунд и ждёт перечитанных данных', () => {
    const onFinish = vi.fn();
    const { rerender } = renderHook(
      ({ running, settled }) => useOnRoundFinished(running, settled, onFinish),
      { initialProps: { running: false, settled: true } }
    );
    // раунд уже сыгран при открытии страницы
    expect(onFinish).not.toHaveBeenCalled();

    rerender({ running: true, settled: true });
    rerender({ running: false, settled: false });
    expect(onFinish).not.toHaveBeenCalled();

    rerender({ running: false, settled: true });
    expect(onFinish).toHaveBeenCalledTimes(1);

    // повторное перечитывание без нового раунда
    rerender({ running: false, settled: false });
    rerender({ running: false, settled: true });
    expect(onFinish).toHaveBeenCalledTimes(1);

    rerender({ running: true, settled: true });
    rerender({ running: false, settled: true });
    expect(onFinish).toHaveBeenCalledTimes(2);
  });
});
