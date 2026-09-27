type Tone = 'yellow' | 'green' | 'blue' | 'red' | 'gray';

interface StatusMeta {
  glyph: string;
  label: string;
  tone: Tone;
}

// Одна таблица статусов на всё приложение. Глифы: ○ в очереди, ◐ идёт,
// ● сыграно или готово, ✕ ошибка, – отменён, ◔ компилируется, ■ завершён.
const STATUS = {
  match: {
    pending: { glyph: '○', label: 'В очереди', tone: 'yellow' },
    running: { glyph: '◐', label: 'Идёт', tone: 'blue' },
    completed: { glyph: '●', label: 'Сыгран', tone: 'green' },
    failed: { glyph: '✕', label: 'Ошибка', tone: 'red' },
    cancelled: { glyph: '–', label: 'Отменён', tone: 'gray' },
  },
  program: {
    compiling: { glyph: '◔', label: 'Компилируется', tone: 'yellow' },
    ready: { glyph: '●', label: 'Готова', tone: 'green' },
    failed: { glyph: '✕', label: 'Ошибка сборки', tone: 'red' },
  },
  tournament: {
    pending: { glyph: '○', label: 'Регистрация', tone: 'yellow' },
    active: { glyph: '◐', label: 'Активный', tone: 'blue' },
    completed: { glyph: '■', label: 'Завершён', tone: 'gray' },
  },
} satisfies Record<string, Record<string, StatusMeta>>;

interface StatusLabelProps {
  entity: keyof typeof STATUS;
  status: string;
  /** Своя подпись (счётчики во множественном числе); глиф и тон из таблицы. */
  label?: string;
  title?: string;
  className?: string;
}

// Бейдж статуса: глиф (только визуально) + подпись. Неизвестный статус
// выводится как есть серым, чтобы новый статус бэкенда не пропал с экрана.
export function StatusLabel({ entity, status, label, title, className = '' }: StatusLabelProps) {
  const meta: StatusMeta | undefined = (STATUS[entity] as Record<string, StatusMeta>)[status];
  return (
    <span className={`badge badge-${meta?.tone ?? 'gray'} font-mono whitespace-nowrap ${className}`} title={title}>
      {meta && <span aria-hidden="true">{meta.glyph} </span>}
      {label ?? meta?.label ?? status}
    </span>
  );
}
