import { Suspense, useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'motion/react';
import { useLocation, useOutlet } from 'react-router-dom';
import { pageTransitionVariants } from './invaderVariants';
import { PageLoader } from '../PageLoader';

const EDITABLE = 'input, textarea, select, [contenteditable="true"]';

// После перехода фокус переносится на h1 новой страницы: иначе он остаётся
// на body, и скринридер не узнаёт, что страница сменилась. Страница может
// грузиться (lazy, данные), поэтому h1 дожидается MutationObserver. Уходящая
// страница ещё в DOM (AnimatePresence), её h1 пропускается.
function useFocusHeadingOnNavigate(pathname: string) {
  // первый рендер и повтор эффекта в StrictMode - не переход
  const prevRef = useRef(pathname);
  useEffect(() => {
    if (prevRef.current === pathname) return;
    prevRef.current = pathname;
    const main = document.getElementById('main-content');
    if (!main) return;
    const stale = main.querySelector('h1');

    const tryFocus = () => {
      const h1 = main.querySelector('h1');
      if (!h1 || h1 === stale) return false;
      // поле с автофокусом или уже начатый ввод не перебиваются
      if (!document.activeElement?.matches(EDITABLE)) {
        h1.tabIndex = -1;
        h1.focus({ preventScroll: true });
      }
      return true;
    };
    if (tryFocus()) return;

    const observer = new MutationObserver(() => {
      if (tryFocus()) observer.disconnect();
    });
    observer.observe(main, { childList: true, subtree: true });
    const timer = setTimeout(() => observer.disconnect(), 10_000);
    return () => {
      observer.disconnect();
      clearTimeout(timer);
    };
  }, [pathname]);
}

export function AnimatedOutlet() {
  const location = useLocation();
  const outlet = useOutlet();
  useFocusHeadingOnNavigate(location.pathname);

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
