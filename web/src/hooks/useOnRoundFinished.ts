import { useEffect, useRef } from 'react';

/**
 * Зовёт onFinish раз за раунд: когда матчи были в работе, а теперь доиграны и
 * зависимые данные перечитаны (settled). Открытие страницы с уже сыгранным
 * раундом не срабатывает.
 */
export function useOnRoundFinished(running: boolean, settled: boolean, onFinish: () => void) {
  const wasRunning = useRef(false);
  const pending = useRef(false);
  const callback = useRef(onFinish);
  useEffect(() => {
    callback.current = onFinish;
  });

  useEffect(() => {
    if (running) {
      wasRunning.current = true;
      return;
    }
    if (wasRunning.current) {
      wasRunning.current = false;
      pending.current = true;
    }
    if (!pending.current || !settled) return;
    pending.current = false;
    callback.current();
  }, [running, settled]);
}
