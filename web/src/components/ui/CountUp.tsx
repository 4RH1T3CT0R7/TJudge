import { useLayoutEffect, useRef } from 'react';
import { animate } from 'motion/react';
import { useMotionPref } from '../../hooks/useMotionPref';

const fmt = (n: number) => n.toLocaleString('ru-RU');

// Число, которое досчитывает от прежнего значения к новому за 0,6 с.
// При reduced motion сразу показывает новое. Текст пишет только эффект:
// React не держит своего текстового узла, который анимация бы подменила.
export function CountUp({ value }: { value: number }) {
  const ref = useRef<HTMLSpanElement>(null);
  const prev = useRef(value);
  const { reduced: reduce } = useMotionPref();

  useLayoutEffect(() => {
    const el = ref.current;
    const from = prev.current;
    prev.current = value;
    if (!el) return;
    if (reduce || from === value) {
      el.textContent = fmt(value);
      return;
    }
    const controls = animate(from, value, {
      duration: 0.6,
      ease: 'easeOut',
      onUpdate: (v) => {
        el.textContent = fmt(Math.round(v));
      },
    });
    return () => {
      controls.stop();
      el.textContent = fmt(value);
    };
  }, [value, reduce]);

  return <span ref={ref} />;
}
