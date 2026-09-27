import type { ReactNode } from 'react';

interface EmptyStateProps {
  /** Что пусто, в виде команды: «ls матчи». */
  command: string;
  /** Пояснение и следующий шаг, выводится после «// ». */
  hint?: ReactNode;
  /** Одно действие: кнопка или ссылка. */
  action?: ReactNode;
}

// Пустое состояние в терминальной грамматике: «$ команда», «// пояснение», действие.
export function EmptyState({ command, hint, action }: EmptyStateProps) {
  return (
    <div className="mx-auto w-fit max-w-full py-12 px-4 font-mono text-sm">
      {/* знаки грамматики только для глаз, скринридер читает сам текст */}
      <p className="text-gray-300">
        <span aria-hidden="true" className="text-primary-400">$ </span>
        {command}
      </p>
      {hint && (
        <p className="mt-2 text-gray-500">
          <span aria-hidden="true">{'// '}</span>
          {hint}
        </p>
      )}
      {action && <div className="mt-5">{action}</div>}
    </div>
  );
}
