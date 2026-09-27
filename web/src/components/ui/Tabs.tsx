import { useEffect, useId, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { handleTabListKeyDown } from './tabKeyboard';

export interface TabItem<T extends string> {
  id: T;
  label: string;
  /** Счётчик после подписи: «Матчи [414]». */
  count?: number;
}

interface TabsProps<T extends string> {
  items: TabItem<T>[];
  active: T;
  onChange: (id: T) => void;
  /** Имя списка вкладок для скринридера. */
  label: string;
  /** Справа от вкладок вне потока (маскот): появление не сдвигает раскладку. */
  aside?: ReactNode;
  /** Содержимое активной вкладки. */
  children: ReactNode;
}

// Вкладки страницы: выбранная отмечена «> », на узком экране список прокручивается
// по горизонтали. Состояние держит вызывающий, обычно через useTabParam (?tab=).
export function Tabs<T extends string>({ items, active, onChange, label, aside, children }: TabsProps<T>) {
  const base = useId();
  const listRef = useRef<HTMLDivElement>(null);
  const [moreRight, setMoreRight] = useState(false);

  // выбранная вкладка не остаётся за краем прокручиваемого списка
  useEffect(() => {
    const list = listRef.current;
    const tab = list?.querySelector<HTMLElement>('[aria-selected="true"]');
    if (!list || !tab) return;
    const right = tab.offsetLeft + tab.offsetWidth;
    if (tab.offsetLeft < list.scrollLeft) list.scrollLeft = tab.offsetLeft;
    else if (right > list.scrollLeft + list.clientWidth) list.scrollLeft = right - list.clientWidth;
  }, [active]);

  // справа есть скрытые вкладки: край списка гаснет, подсказывая прокрутку
  useEffect(() => {
    const list = listRef.current;
    if (!list) return;
    const update = () => setMoreRight(list.scrollLeft + list.clientWidth < list.scrollWidth - 1);
    const raf = requestAnimationFrame(update);
    list.addEventListener('scroll', update, { passive: true });
    window.addEventListener('resize', update);
    return () => {
      cancelAnimationFrame(raf);
      list.removeEventListener('scroll', update);
      window.removeEventListener('resize', update);
    };
  }, [items.length]);

  return (
    <>
      <div className="relative mb-6">
        <div
          ref={listRef}
          role="tablist"
          aria-label={label}
          onKeyDown={handleTabListKeyDown}
          // линия под вкладками тенью: рамку прокручиваемого списка перекрыть нельзя, тень рисуется под кнопками
          className={`relative flex overflow-x-auto font-mono text-sm shadow-[inset_0_-1px_0_var(--color-gray-800)] [scrollbar-width:thin] ${
            moreRight ? '[mask-image:linear-gradient(to_right,black_calc(100%-3rem),transparent)]' : ''
          }`}
        >
          {items.map((tab) => {
            const selected = tab.id === active;
            return (
              <button
                key={tab.id}
                id={`${base}-tab-${tab.id}`}
                type="button"
                role="tab"
                aria-selected={selected}
                aria-controls={`${base}-panel`}
                tabIndex={selected ? 0 : -1}
                onClick={() => onChange(tab.id)}
                className={`shrink-0 whitespace-nowrap px-3 py-2.5 border-b-2 transition-colors ${
                  selected ? 'border-primary-400 text-gray-100' : 'border-transparent text-gray-400 hover:text-gray-100'
                }`}
              >
                <span aria-hidden="true" className={selected ? 'text-primary-400' : 'invisible'}>{'> '}</span>
                {tab.label}
                {tab.count !== undefined && <span className="ml-1.5 text-gray-500">[{tab.count}]</span>}
              </button>
            );
          })}
        </div>
        {aside}
      </div>
      <div role="tabpanel" id={`${base}-panel`} aria-labelledby={`${base}-tab-${active}`} tabIndex={0}>
        {children}
      </div>
    </>
  );
}
