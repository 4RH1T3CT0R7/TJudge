interface SegmentedProps<T extends string> {
  options: readonly { value: T; label: string }[];
  value: T;
  onChange: (value: T) => void;
  /** Имя группы для скринридера. */
  label: string;
}

// Переключатель вида из нескольких кнопок (не вкладки: содержимое страницы то же,
// меняется представление). Выбранная кнопка — aria-pressed.
export function Segmented<T extends string>({ options, value, onChange, label }: SegmentedProps<T>) {
  return (
    <div role="group" aria-label={label} className="inline-flex rounded border border-gray-700 p-0.5 font-mono text-sm">
      {options.map((option) => {
        const pressed = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={pressed}
            onClick={() => onChange(option.value)}
            className={`px-3 py-1.5 rounded transition-colors ${
              pressed ? 'bg-gray-800 text-primary-300' : 'text-gray-400 hover:text-gray-100'
            }`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
