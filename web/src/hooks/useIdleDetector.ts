import { useState, useEffect, useRef, useCallback } from 'react';

export type IdleStage = 'active' | 'idle30s' | 'idle1m' | 'idle5m';

interface UseIdleDetectorOptions {
  enabled?: boolean;
}

export function useIdleDetector({ enabled = true }: UseIdleDetectorOptions = {}) {
  const [stage, setStage] = useState<IdleStage>('active');
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  const resetTimers = useCallback(() => {
    timersRef.current.forEach(clearTimeout);
    timersRef.current = [];
    setStage('active');

    if (!enabled) return;

    timersRef.current.push(
      setTimeout(() => setStage('idle30s'), 30_000),
      setTimeout(() => setStage('idle1m'), 60_000),
      setTimeout(() => setStage('idle5m'), 300_000),
    );
  }, [enabled]);

  useEffect(() => {
    if (!enabled) return;

    resetTimers();

    const onActivity = () => resetTimers();
    window.addEventListener('mousemove', onActivity, { passive: true });
    window.addEventListener('keydown', onActivity, { passive: true });
    window.addEventListener('click', onActivity, { passive: true });
    window.addEventListener('scroll', onActivity, { passive: true });

    return () => {
      timersRef.current.forEach(clearTimeout);
      window.removeEventListener('mousemove', onActivity);
      window.removeEventListener('keydown', onActivity);
      window.removeEventListener('click', onActivity);
      window.removeEventListener('scroll', onActivity);
    };
  }, [enabled, resetTimers]);

  return stage;
}
