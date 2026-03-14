import { useEffect, useRef, useState } from 'react';
import { SpaceInvader } from '../SpaceInvader';
import { QuestEnvironment } from './QuestEnvironment';
import type { QuestState } from '../../hooks/useQuestState';

// Color transform CSS filters
const COLOR_FILTERS: Record<string, string> = {
  fire: 'hue-rotate(-30deg) saturate(2) brightness(1.3)',
  ice: 'hue-rotate(180deg) saturate(1.5) brightness(1.2)',
  ghost: 'opacity(0.5) brightness(2) blur(1px)',
  rainbow: '', // handled via animation class
};

interface QuestInvaderProps {
  state: QuestState;
}

export function QuestInvader({ state }: QuestInvaderProps) {
  const { invaderPose, invaderMood, invaderSpeech, invaderTransform, invaderJump, escaping } = state;
  const [jumpTrigger, setJumpTrigger] = useState(false);
  const [shakeTrigger, setShakeTrigger] = useState(false);
  const prevJumpRef = useRef(invaderJump);
  const containerRef = useRef<HTMLDivElement>(null);

  // Trigger jump animation — timer-driven state transition
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    if (invaderJump !== prevJumpRef.current) {
      prevJumpRef.current = invaderJump;
      setJumpTrigger(true);
      const t = setTimeout(() => setJumpTrigger(false), 600);
      return () => clearTimeout(t);
    }
  }, [invaderJump]);

  // Shake on attack — timer-driven state transition
  useEffect(() => {
    if (invaderPose === 'attack') {
      setShakeTrigger(true);
      const t = setTimeout(() => setShakeTrigger(false), 500);
      return () => clearTimeout(t);
    }
  }, [invaderPose]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // Eye override based on mood
  const eyeOverride = (() => {
    switch (invaderMood) {
      case 'sleepy': return 'closed' as const;
      case 'scared': return 'sad' as const;
      case 'angry': return 'wide' as const;
      case 'happy': return 'wide' as const;
      default: return null;
    }
  })();

  // Color filter
  const colorFilter = invaderTransform ? COLOR_FILTERS[invaderTransform] : undefined;
  const isRainbow = invaderTransform === 'rainbow';

  const inQuest = state.level > 0;

  return (
    <div
      ref={containerRef}
      className={`flex items-center justify-center ${isRainbow ? 'animate-rainbow-hue' : ''}`}
      style={{
        minWidth: inQuest ? '280px' : '200px',
        minHeight: inQuest ? '300px' : '220px',
        position: escaping ? 'fixed' : 'relative',
        zIndex: escaping ? 100 : 1,
        transition: 'all 0.5s ease-out',
        ...(escaping
          ? {
              top: '50%',
              left: '50%',
              transform: 'translate(-50%, -50%)',
              animation: 'page-escape 3s ease-in-out forwards',
            }
          : {}),
      }}
    >
      <QuestEnvironment level={state.level} invaderPose={invaderPose}>
        <SpaceInvader
          size="md"
          controlledPose={invaderPose}
          eyeOverride={eyeOverride}
          shake={shakeTrigger}
          jump={jumpTrigger}
          speechBubble={invaderSpeech}
          colorFilter={colorFilter}
        />
      </QuestEnvironment>
    </div>
  );
}
