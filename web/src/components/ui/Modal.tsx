import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { KeyboardEvent, ReactNode } from 'react';
import { XMarkIcon } from '../icons';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children: ReactNode;
  /** Tailwind-класс максимальной ширины контента. */
  maxWidth?: string;
  /** false у форм: случайный клик мимо панели не стирает введённое. */
  closeOnBackdrop?: boolean;
}

// Открытые модалки в порядке открытия: Escape закрывает только верхнюю,
// подтверждение поверх формы не должно закрыть заодно и форму. Слушатель один
// на все модалки: у отдельных порядок вызова меняется при перерисовках, а
// закрытая верхняя успевает уйти со стека до вызова слушателя нижней.
const openStack: { current: () => void }[] = [];

function closeTop(e: { key: string }) {
  if (e.key === 'Escape') openStack[openStack.length - 1]?.current();
}

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

// Tab и Shift+Tab ходят по кругу внутри диалога.
function trapTab(e: KeyboardEvent<HTMLDivElement>) {
  if (e.key !== 'Tab') return;
  const items = e.currentTarget.querySelectorAll<HTMLElement>(FOCUSABLE);
  if (items.length === 0) {
    e.preventDefault();
    return;
  }
  const first = items[0];
  const last = items[items.length - 1];
  const active = document.activeElement;
  if (e.shiftKey && (active === first || active === e.currentTarget)) {
    e.preventDefault();
    last.focus();
  } else if (!e.shiftKey && active === last) {
    e.preventDefault();
    first.focus();
  }
}

// Общая модалка на классах .modal-backdrop/.modal-content (index.css):
// клик по фону (если не отключён) и Escape закрывают, клик по контенту — нет.
// Фокус уходит в диалог и после закрытия возвращается на элемент, который его открыл.
// Рендер в body: иначе контекст наложения страницы оставляет шапку поверх фона.
export function Modal({ open, onClose, title, children, maxWidth = 'max-w-md', closeOnBackdrop = true }: ModalProps) {
  const onCloseRef = useRef(onClose);
  useLayoutEffect(() => {
    onCloseRef.current = onClose;
  });
  useEffect(() => {
    if (!open) return;
    if (openStack.length === 0) document.addEventListener('keydown', closeTop);
    openStack.push(onCloseRef);
    return () => {
      openStack.splice(openStack.indexOf(onCloseRef), 1);
      if (openStack.length === 0) document.removeEventListener('keydown', closeTop);
    };
  }, [open]);
  const titleId = useId();
  const panelRef = useRef<HTMLDivElement>(null);
  // выделение текста в поле, отпущенное над фоном, тоже даёт click по фону
  const pressedOnBackdrop = useRef(false);

  // Открывший элемент запоминается при рендере: autoFocus внутри диалога
  // срабатывает раньше эффектов и уже успевает перевести фокус.
  const [opener, setOpener] = useState<Element | null>(() => (open ? document.activeElement : null));
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setOpener(document.activeElement);
  }

  useEffect(() => {
    if (!open) return;
    const panel = panelRef.current;
    if (panel && !panel.contains(document.activeElement)) panel.focus();
    return () => {
      if (opener instanceof HTMLElement) opener.focus();
    };
  }, [open, opener]);

  if (!open) return null;

  return createPortal(
    <div
      className="modal-backdrop"
      onMouseDown={(e) => { pressedOnBackdrop.current = e.target === e.currentTarget; }}
      onClick={(e) => {
        if (closeOnBackdrop && pressedOnBackdrop.current && e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title !== undefined ? titleId : undefined}
        tabIndex={-1}
        className={`modal-content w-full ${maxWidth} max-h-[calc(100dvh-2rem)] overflow-y-auto p-6 m-4 outline-none`}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={trapTab}
      >
        {title !== undefined && (
          <div className="flex items-center justify-between mb-6">
            <h2 id={titleId} className="text-xl font-bold text-gray-100">{title}</h2>
            <button
              onClick={onClose}
              aria-label="Закрыть"
              className="p-2 hover:bg-gray-800 rounded-lg transition-colors"
            >
              <XMarkIcon />
            </button>
          </div>
        )}
        {children}
      </div>
    </div>,
    document.body,
  );
}
