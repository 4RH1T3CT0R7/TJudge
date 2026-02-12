import { motion } from 'motion/react';
import { staggerContainerVariants, staggerItemVariants } from './invaderVariants';
import type { ReactNode } from 'react';

interface StaggerListProps {
  children: ReactNode;
  className?: string;
}

export function StaggerList({ children, className = '' }: StaggerListProps) {
  return (
    <motion.div
      className={className}
      variants={staggerContainerVariants}
      initial="hidden"
      animate="visible"
    >
      {children}
    </motion.div>
  );
}

export function StaggerItem({ children, className = '' }: StaggerListProps) {
  return (
    <motion.div className={className} variants={staggerItemVariants}>
      {children}
    </motion.div>
  );
}
