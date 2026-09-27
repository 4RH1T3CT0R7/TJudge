import { motion } from 'motion/react';
import { SpaceInvader, type SpaceInvaderProps } from '../SpaceInvader';
import { invaderEnterVariants } from './invaderVariants';

export function InvaderPresence(props: SpaceInvaderProps) {
  return (
    <motion.div variants={invaderEnterVariants} initial="hidden" animate="visible">
      <SpaceInvader {...props} />
    </motion.div>
  );
}
