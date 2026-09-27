import { useCallback, useRef } from 'react';

// Своя строка на телефоне: длинная таблица уходит за экран, поэтому при первом
// появлении строка прокручивается в поле зрения. На широком экране не трогается.
export function useRevealOnMobile<T extends HTMLElement>() {
  const done = useRef(false);
  return useCallback((el: T | null) => {
    if (!el || done.current) return;
    done.current = true;
    if (!window.matchMedia?.('(max-width: 639px)').matches) return;
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    el.scrollIntoView({ block: 'nearest', behavior: reduce ? 'auto' : 'smooth' });
  }, []);
}
