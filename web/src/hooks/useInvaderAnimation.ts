import { useCallback, useRef } from 'react';
import type { InvaderPose } from '../components/SpaceInvader';

interface AnimationStep {
  pose: InvaderPose;
  speech?: string | null;
  eye?: 'closed' | 'sad' | 'wide' | null;
  shake?: boolean;
  jump?: boolean;
  duration: number;
}

/**
 * Imperative animation sequencer for complex pose transitions.
 * Returns a `play` function that takes a sequence of steps and a `setPose` callback.
 */
export function useInvaderAnimation() {
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  const cancel = useCallback(() => {
    timersRef.current.forEach(clearTimeout);
    timersRef.current = [];
  }, []);

  const play = useCallback((
    steps: AnimationStep[],
    onStep: (step: Omit<AnimationStep, 'duration'>) => void,
    onComplete?: () => void,
  ) => {
    cancel();

    let elapsed = 0;
    for (const step of steps) {
      const { duration, ...rest } = step;
      const t = setTimeout(() => onStep(rest), elapsed);
      timersRef.current.push(t);
      elapsed += duration;
    }

    if (onComplete) {
      const t = setTimeout(onComplete, elapsed);
      timersRef.current.push(t);
    }
  }, [cancel]);

  return { play, cancel };
}
