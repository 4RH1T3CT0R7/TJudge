import { useId } from 'react';
import type { ReactNode } from 'react';

export interface FieldControlProps {
  id: string;
  'aria-describedby'?: string;
  'aria-invalid'?: true;
  'aria-required'?: true;
}

interface FieldProps {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  /** Обязательное поле: звёздочка в подписи видна глазу, скринридер слышит aria-required. */
  required?: boolean;
  /** Контрол получает id и aria-атрибуты: {(p) => <input {...p} className="input" />}. */
  children: (control: FieldControlProps) => ReactNode;
  className?: string;
}

// Поле формы: подпись связана с контролом через htmlFor, подсказка и ошибка —
// через aria-describedby; появившаяся ошибка озвучивается сразу.
export function Field({ label, hint, error, required, children, className }: FieldProps) {
  const id = useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined;

  return (
    <div className={className}>
      <label htmlFor={id} className="block mb-1.5 text-sm font-medium text-gray-300">
        {label}
        {required && <span aria-hidden="true"> *</span>}
      </label>
      {children({
        id,
        'aria-describedby': describedBy,
        'aria-invalid': error ? true : undefined,
        'aria-required': required || undefined,
      })}
      {hint && (
        <p id={hintId} className="mt-1.5 text-xs text-gray-500">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} role="alert" className="mt-1.5 text-sm text-red-400">
          {error}
        </p>
      )}
    </div>
  );
}
