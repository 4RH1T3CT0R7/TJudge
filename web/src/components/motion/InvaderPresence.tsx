import { m } from 'motion/react';
import { SpaceInvader, type SpaceInvaderProps } from '../SpaceInvader';
import { invaderEnterVariants } from './invaderVariants';

export function InvaderPresence(props: SpaceInvaderProps) {
  return (
    <m.div variants={invaderEnterVariants} initial="hidden" animate="visible">
      <SpaceInvader {...props} />
    </m.div>
  );
}
