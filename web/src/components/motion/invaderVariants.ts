import type { Variants } from 'motion/react';
import { DUR, EASE_OUT } from './tokens';

// Маскот выезжает сбоку на единственной пружине
export const invaderEnterVariants: Variants = {
  hidden: { opacity: 0, x: 40 },
  visible: {
    opacity: 1,
    x: 0,
    transition: { type: 'spring', stiffness: 260, damping: 20 },
  },
};

export const pageTransitionVariants: Variants = {
  initial: { opacity: 0, y: 8 },
  animate: {
    opacity: 1,
    y: 0,
    transition: { duration: DUR.base, ease: EASE_OUT },
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
