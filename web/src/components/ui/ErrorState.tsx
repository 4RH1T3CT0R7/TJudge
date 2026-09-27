import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';

interface ErrorStateProps {
  message: string;
  onRetry?: () => void;
  /** Дополнительные действия рядом с «Повторить»: ссылка назад и т. п. */
  children?: ReactNode;
}

// Ошибка загрузки в терминальной грамматике: «stderr: текст» и кнопка повтора.
export function ErrorState({ message, onRetry, children }: ErrorStateProps) {
  const [hadFocus, setHadFocus] = useState(false);

  // удачный повтор убирает блок вместе с кнопкой в фокусе: фокус переходит
  // на h1 страницы, а не теряется на body
  useEffect(() => {
    if (!hadFocus) return;
    return () => {
      if (document.activeElement && document.activeElement !== document.body) return;
      const h1 = document.querySelector<HTMLElement>('#main-content h1');
      if (!h1) return;
      h1.tabIndex = -1;
      h1.focus({ preventScroll: true });
    };
  }, [hadFocus]);

  return (
    <div role="alert" onFocus={() => setHadFocus(true)} className="mx-auto w-fit max-w-full py-12 px-4 font-mono text-sm">
      <p>
        <span aria-hidden="true" className="text-red-400">stderr: </span>
        <span className="sr-only">Ошибка: </span>
        <span className="text-gray-200">{message}</span>
      </p>
      {(onRetry || children) && (
        <div className="mt-5 flex flex-wrap gap-3">
          {onRetry && (
            <button type="button" onClick={onRetry} className="btn btn-secondary">
              Повторить
            </button>
          )}
          {children}
        </div>
      )}
    </div>
  );
}
