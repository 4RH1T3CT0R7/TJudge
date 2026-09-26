import { useEffect, useRef } from 'react';

/**
 * Зовёт onFinish раз за раунд: когда матчи были в работе, а теперь доиграны.
 * Открытие страницы с уже сыгранным раундом не срабатывает.
 */
export function useOnRoundFinished(running: boolean, onFinish: () => void) {
  const wasRunning = useRef(false);
  const callback = useRef(onFinish);
  useEffect(() => {
    callback.current = onFinish;
  });

  useEffect(() => {
    if (running) {
      wasRunning.current = true;
    } else if (wasRunning.current) {
      wasRunning.current = false;
      callback.current();
    }
  }, [running]);
}
