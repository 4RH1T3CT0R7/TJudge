import { AnimatePresence, motion } from 'motion/react';
import { SpaceInvader, type SpaceInvaderProps } from '../SpaceInvader';
import { entranceVariants, slideLeftVariants, peekVariants, popVariants } from './invaderVariants';
import type { Variants } from 'motion/react';

type EntranceType = 'pop' | 'slideLeft' | 'peek' | 'default';

const variantMap: Record<EntranceType, Variants> = {
  default: entranceVariants,
  pop: popVariants,
  slideLeft: slideLeftVariants,
  peek: peekVariants,
};

interface InvaderPresenceProps extends SpaceInvaderProps {
  show?: boolean;
  entrance?: EntranceType;
  motionKey?: string;
}

export function InvaderPresence({
  show = true,
  entrance = 'default',
  motionKey = 'invader',
  ...invaderProps
}: InvaderPresenceProps) {
  const variants = variantMap[entrance];

  return (
    <AnimatePresence mode="wait">
      {show && (
        <motion.div
          key={motionKey}
          variants={variants}
          initial="hidden"
          animate="visible"
          exit="exit"
        >
          <SpaceInvader {...invaderProps} />
        </motion.div>
      )}
    </AnimatePresence>
  );
}
