import type { KeyboardEvent } from 'react';

// Клавиатура для role="tablist" (WAI-ARIA tabs): стрелки влево/вправо, Home и End
// переводят фокус на соседнюю вкладку и сразу её выбирают. Кнопкам вкладок
// нужны role="tab", aria-selected и tabIndex={выбрана ? 0 : -1}.
export function handleTabListKeyDown(e: KeyboardEvent<HTMLElement>) {
  const tabs = Array.from(e.currentTarget.querySelectorAll<HTMLElement>('[role="tab"]'));
  const current = tabs.indexOf(document.activeElement as HTMLElement);
  if (current === -1) return;
  const target = { ArrowRight: current + 1, ArrowLeft: current - 1, Home: 0, End: tabs.length - 1 }[e.key];
  if (target === undefined) return;
  e.preventDefault();
  const tab = tabs[(target + tabs.length) % tabs.length];
  tab.focus();
  tab.click();
}
