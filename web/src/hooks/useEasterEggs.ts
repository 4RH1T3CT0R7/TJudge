import { useEffect, useRef, useCallback, useState } from 'react';

const KONAMI_CODE = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a'];

export function useKonamiCode(onActivate: () => void) {
  const indexRef = useRef(0);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA') return;

      if (e.key === KONAMI_CODE[indexRef.current]) {
        indexRef.current++;
        if (indexRef.current === KONAMI_CODE.length) {
          indexRef.current = 0;
          onActivate();
        }
      } else {
        indexRef.current = 0;
      }
    };

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onActivate]);
}

export function useSequenceTyping(sequence: string, onActivate: () => void) {
  const indexRef = useRef(0);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA') return;

      if (e.key.toLowerCase() === sequence[indexRef.current]) {
        indexRef.current++;
        if (indexRef.current === sequence.length) {
          indexRef.current = 0;
          onActivate();
        }
      } else {
        indexRef.current = 0;
      }
    };

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [sequence, onActivate]);
}

export function useRapidClicks(threshold: number, onActivate: () => void) {
  const countRef = useRef(0);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const onClick = useCallback(() => {
    countRef.current++;
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => { countRef.current = 0; }, 2000);

    if (countRef.current >= threshold) {
      countRef.current = 0;
      onActivate();
    }
  }, [threshold, onActivate]);

  return onClick;
}

export function useDoubleClickText(onActivate: () => void) {
  const onDoubleClick = useCallback(() => {
    onActivate();
  }, [onActivate]);

  return onDoubleClick;
}

export function useGodMode() {
  const [active, setActive] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  const activate = useCallback(() => {
    setActive(true);
    clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => setActive(false), 30_000);
  }, []);

  return { godMode: active, activateGodMode: activate };
}
