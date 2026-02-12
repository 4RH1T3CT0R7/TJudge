import { useState, useCallback, useRef } from 'react';
import type { InvaderPose } from '../components/SpaceInvader';

interface InvaderReaction {
  pose: InvaderPose;
  speech: string | null;
  eye: 'closed' | 'sad' | 'wide' | null;
  shake: boolean;
  jump: boolean;
}

const DEFAULT_REACTION: InvaderReaction = {
  pose: 'idle',
  speech: null,
  eye: null,
  shake: false,
  jump: false,
};

type ReactionSequence = {
  poses: { pose: InvaderPose; duration: number }[];
  speeches: string[];
  eye: InvaderReaction['eye'];
  shake?: boolean;
  jump?: boolean;
};

function pickRandom<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)];
}

/**
 * Local invader reactor hook — for page-level invader reactions.
 * Use this when you don't need the global InvaderContext.
 */
export function useInvaderReactor(initialPose: InvaderPose = 'idle', initialSpeech: string | null = null) {
  const [reaction, setReaction] = useState<InvaderReaction>({
    ...DEFAULT_REACTION,
    pose: initialPose,
    speech: initialSpeech,
  });
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  const clearTimers = useCallback(() => {
    timersRef.current.forEach(clearTimeout);
    timersRef.current = [];
  }, []);

  const runSequence = useCallback((seq: ReactionSequence) => {
    clearTimers();

    const speech = pickRandom(seq.speeches);
    setReaction({
      pose: seq.poses[0].pose,
      speech,
      eye: seq.eye,
      shake: seq.shake ?? false,
      jump: seq.jump ?? false,
    });

    // Clear shake/jump
    const t0 = setTimeout(() => {
      setReaction(prev => ({ ...prev, shake: false, jump: false }));
    }, 600);
    timersRef.current.push(t0);

    // Sequence through poses
    let elapsed = seq.poses[0].duration;
    for (let i = 1; i < seq.poses.length; i++) {
      const p = seq.poses[i];
      const t = setTimeout(() => {
        setReaction(prev => ({ ...prev, pose: p.pose }));
      }, elapsed);
      timersRef.current.push(t);
      elapsed += p.duration;
    }

    // Return to default
    const tEnd = setTimeout(() => {
      setReaction(prev => ({ ...prev, pose: initialPose, speech: initialSpeech, eye: null, shake: false, jump: false }));
    }, elapsed);
    timersRef.current.push(tEnd);
  }, [clearTimers, initialPose, initialSpeech]);

  const setPose = useCallback((pose: InvaderPose, speech?: string | null, eye?: InvaderReaction['eye']) => {
    setReaction(prev => ({
      ...prev,
      pose,
      speech: speech !== undefined ? speech : prev.speech,
      eye: eye !== undefined ? eye : prev.eye,
    }));
  }, []);

  const triggerShake = useCallback(() => {
    setReaction(prev => ({ ...prev, shake: true }));
    const t = setTimeout(() => setReaction(prev => ({ ...prev, shake: false })), 500);
    timersRef.current.push(t);
  }, []);

  const triggerJump = useCallback(() => {
    setReaction(prev => ({ ...prev, jump: true }));
    const t = setTimeout(() => setReaction(prev => ({ ...prev, jump: false })), 600);
    timersRef.current.push(t);
  }, []);

  return {
    ...reaction,
    setPose,
    runSequence,
    triggerShake,
    triggerJump,
    clearTimers,
  };
}
