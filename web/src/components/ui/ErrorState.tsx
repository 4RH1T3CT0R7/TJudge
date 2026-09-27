import type { ReactNode } from 'react';

interface ErrorStateProps {
  message: string;
  onRetry?: () => void;
  /** Дополнительные действия рядом с «Повторить»: ссылка назад и т. п. */
  children?: ReactNode;
}

// Ошибка загрузки в терминальной грамматике: «stderr: текст» и кнопка повтора.
export function ErrorState({ message, onRetry, children }: ErrorStateProps) {
  return (
    <div role="alert" className="mx-auto w-fit max-w-full py-12 px-4 font-mono text-sm">
      <p>
        <span className="text-red-400">stderr:</span> <span className="text-gray-200">{message}</span>
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
