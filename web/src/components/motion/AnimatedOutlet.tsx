import { Suspense } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { useLocation, useOutlet } from 'react-router-dom';
import { pageTransitionVariants } from './invaderVariants';
import { PageLoader } from '../PageLoader';

export function AnimatedOutlet() {
  const location = useLocation();
  const outlet = useOutlet();

  return (
    <AnimatePresence mode="wait" initial={false}>
      <motion.div
        key={location.pathname}
        className="flex-1 flex flex-col"
        variants={pageTransitionVariants}
        initial="initial"
        animate="animate"
        exit="exit"
      >
        <Suspense fallback={<PageLoader />}>
          {outlet}
        </Suspense>
      </motion.div>
    </AnimatePresence>
  );
}
