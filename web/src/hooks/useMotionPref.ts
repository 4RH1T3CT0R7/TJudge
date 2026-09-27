import { useSyncExternalStore } from 'react';

// Одна настройка движения на всё приложение: системная prefers-reduced-motion,
// которую переопределяет переключатель в подвале (хранится в localStorage).
// Итог пишется в <html data-motion="reduced">: по нему CSS гасит анимации и переходы,
// MotionConfig выключает анимации motion, а таймерная хореография (печать, маскот,
// PixelGrid) читает useMotionPref() или isMotionReduced().
const KEY = 'motion';
const media = typeof window !== 'undefined' ? window.matchMedia?.('(prefers-reduced-motion: reduce)') : undefined;
const listeners = new Set<() => void>();

function readOverride(): boolean | null {
  try {
    const v = localStorage.getItem(KEY);
    return v === 'reduced' ? true : v === 'full' ? false : null;
  } catch {
    return null;
  }
}

let override = readOverride();
let reduced = false;

function apply() {
  reduced = override ?? !!media?.matches;
  if (reduced) document.documentElement.dataset.motion = 'reduced';
  else delete document.documentElement.dataset.motion;
  listeners.forEach((l) => l());
}

if (typeof document !== 'undefined') {
  apply();
  media?.addEventListener?.('change', apply);
}

export const isMotionReduced = () => reduced;

// Выбор, совпавший с системной настройкой, снимает переопределение: дальше снова действует система
export function setMotionReduced(value: boolean) {
  override = value === !!media?.matches ? null : value;
  try {
    if (override === null) localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, value ? 'reduced' : 'full');
  } catch {
    // без хранилища выбор живёт до перезагрузки
  }
  apply();
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}

export function useMotionPref() {
  const value = useSyncExternalStore(subscribe, isMotionReduced, isMotionReduced);
  return { reduced: value, setReduced: setMotionReduced };
}
