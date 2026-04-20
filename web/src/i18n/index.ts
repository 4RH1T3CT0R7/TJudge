// Lightweight i18n без внешней зависимости.
//
// Предоставляет t(key) для ключей вида "auth.login_title", переключение
// языка через `setLocale()` + event-based re-render (простой store).
// При необходимости легко заменить на react-i18next - API совместимый.

import { useEffect, useState } from 'react';
import ru from './locales/ru.json';
import en from './locales/en.json';

type Dict = Record<string, Record<string, string>>;

export type Locale = 'ru' | 'en';

const dicts: Record<Locale, Dict> = {
  ru: ru as Dict,
  en: en as Dict,
};

const STORAGE_KEY = 'tjudge.locale';
const CHANGE_EVENT = 'tjudge:locale-change';

function detectLocale(): Locale {
  const saved = typeof localStorage !== 'undefined' ? localStorage.getItem(STORAGE_KEY) : null;
  if (saved === 'ru' || saved === 'en') return saved;
  const nav = typeof navigator !== 'undefined' ? navigator.language : '';
  return nav.toLowerCase().startsWith('en') ? 'en' : 'ru';
}

let currentLocale: Locale = detectLocale();

/**
 * t(key) возвращает строку по ключу "section.name". Если ключ не найден,
 * возвращает сам key (visible в UI, подсказывая добавить перевод).
 */
export function t(key: string): string {
  const [section, name] = key.split('.', 2);
  if (!section || !name) return key;
  const dict = dicts[currentLocale];
  return dict?.[section]?.[name] ?? key;
}

/**
 * setLocale сохраняет язык и триггерит re-render подписчиков через CustomEvent.
 * Перезагрузка страницы не требуется.
 */
export function setLocale(loc: Locale) {
  currentLocale = loc;
  try {
    localStorage.setItem(STORAGE_KEY, loc);
  } catch {
    // приват-режим
  }
  window.dispatchEvent(new CustomEvent(CHANGE_EVENT));
}

export function getLocale(): Locale {
  return currentLocale;
}

/**
 * useTranslation - React hook: возвращает {t, locale, setLocale} и
 * подписывается на смену языка (вызывает re-render).
 */
export function useTranslation() {
  const [, force] = useState(0);
  useEffect(() => {
    const handler = () => force((n) => n + 1);
    window.addEventListener(CHANGE_EVENT, handler);
    return () => window.removeEventListener(CHANGE_EVENT, handler);
  }, []);
  return {
    t,
    locale: currentLocale,
    setLocale,
  };
}
