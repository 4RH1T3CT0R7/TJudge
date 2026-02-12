import { useState, useEffect, useRef } from 'react';

interface ScrollInvaderState {
  scrollY: number;
  scrollProgress: number;
  isScrollingDown: boolean;
}

/**
 * Tracks scroll position and progress within a container or the window.
 */
export function useScrollInvader(containerRef?: React.RefObject<HTMLElement | null>) {
  const [state, setState] = useState<ScrollInvaderState>({
    scrollY: 0,
    scrollProgress: 0,
    isScrollingDown: false,
  });
  const prevScrollRef = useRef(0);

  useEffect(() => {
    const target = containerRef?.current || window;
    const getScrollY = () => containerRef?.current
      ? containerRef.current.scrollTop
      : window.scrollY;
    const getMaxScroll = () => containerRef?.current
      ? containerRef.current.scrollHeight - containerRef.current.clientHeight
      : document.documentElement.scrollHeight - window.innerHeight;

    let raf = 0;
    const onScroll = () => {
      if (raf) return;
      raf = requestAnimationFrame(() => {
        raf = 0;
        const y = getScrollY();
        const max = getMaxScroll();
        const progress = max > 0 ? Math.min(1, y / max) : 0;
        const isDown = y > prevScrollRef.current;
        prevScrollRef.current = y;
        setState({ scrollY: y, scrollProgress: progress, isScrollingDown: isDown });
      });
    };

    target.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      target.removeEventListener('scroll', onScroll);
      if (raf) cancelAnimationFrame(raf);
    };
  }, [containerRef]);

  return state;
}
