import type { Variants, Transition } from 'motion/react';

const springBounce: Transition = {
  type: 'spring',
  stiffness: 400,
  damping: 17,
};

const springGentle: Transition = {
  type: 'spring',
  stiffness: 260,
  damping: 20,
};

export const entranceVariants: Variants = {
  hidden: { opacity: 0, scale: 0.6, y: 20 },
  visible: {
    opacity: 1,
    scale: 1,
    y: 0,
    transition: springBounce,
  },
  exit: {
    opacity: 0,
    scale: 0.6,
    y: -20,
    transition: { duration: 0.2 },
  },
};

export const slideLeftVariants: Variants = {
  hidden: { opacity: 0, x: 40 },
  visible: {
    opacity: 1,
    x: 0,
    transition: springGentle,
  },
  exit: {
    opacity: 0,
    x: 40,
    transition: { duration: 0.15 },
  },
};

export const slideUpVariants: Variants = {
  hidden: { opacity: 0, y: 20 },
  visible: {
    opacity: 1,
    y: 0,
    transition: springGentle,
  },
  exit: {
    opacity: 0,
    y: -10,
    transition: { duration: 0.15 },
  },
};

export const peekVariants: Variants = {
  hidden: { opacity: 0, x: 30, scale: 0.8 },
  visible: {
    opacity: 1,
    x: 0,
    scale: 1,
    transition: { ...springBounce, delay: 0.3 },
  },
  exit: {
    opacity: 0,
    x: 30,
    scale: 0.8,
    transition: { duration: 0.15 },
  },
};

export const popVariants: Variants = {
  hidden: { opacity: 0, scale: 0 },
  visible: {
    opacity: 1,
    scale: 1,
    transition: springBounce,
  },
  exit: {
    opacity: 0,
    scale: 0,
    transition: { duration: 0.15 },
  },
};

export const pageTransitionVariants: Variants = {
  initial: { opacity: 0, y: 8 },
  animate: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.2, ease: 'easeOut' },
  },
  // Exit мгновенный: с mode="wait" AnimatePresence ждёт завершения exit перед
  // монтированием новой страницы. Любая ненулевая длительность создаёт «провал
  // в пустоту» (старая уже исчезла, новая ещё не появилась) — это и есть
  // промаргивание при каждом переходе. Мгновенный exit убирает пустой кадр,
  // остаётся только чистое появление новой страницы.
  exit: {
    opacity: 0,
    transition: { duration: 0 },
  },
};

export const staggerContainerVariants: Variants = {
  hidden: { opacity: 0 },
  visible: {
    opacity: 1,
    transition: {
      staggerChildren: 0.06,
      delayChildren: 0.1,
    },
  },
};

export const staggerItemVariants: Variants = {
  hidden: { opacity: 0, y: 16 },
  visible: {
    opacity: 1,
    y: 0,
    transition: springGentle,
  },
};

export const celebrateVariants: Variants = {
  idle: { scale: 1, rotate: 0 },
  celebrate: {
    scale: [1, 1.15, 1],
    rotate: [0, -5, 5, 0],
    transition: { duration: 0.6, ease: 'easeInOut' },
  },
};

export const jumpVariants: Variants = {
  idle: { y: 0 },
  jump: {
    y: [0, -30, -5, -15, 0],
    transition: { duration: 0.6, ease: 'easeOut' },
  },
};

export const shakeVariants: Variants = {
  idle: { x: 0 },
  shake: {
    x: [0, -6, 6, -4, 4, -2, 2, 0],
    transition: { duration: 0.5 },
  },
};
