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

// Переход к разделу внутри страницы: прокрутка к заголовку и фокус на нём,
// чтобы и глаз, и клавиатура, и скринридер оказались у начала раздела.
export function revealAndFocus(el: HTMLElement | null) {
  if (!el) return;
  // заголовок уже на экране (ниже шапки) - страница не дёргается
  const pad = parseFloat(getComputedStyle(document.documentElement).scrollPaddingTop) || 0;
  const r = el.getBoundingClientRect();
  if (r.top < pad || r.bottom > window.innerHeight) {
    const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    el.scrollIntoView({ block: 'start', behavior: reduce ? 'auto' : 'smooth' });
  }
  el.tabIndex = -1;
  el.focus({ preventScroll: true });
}
