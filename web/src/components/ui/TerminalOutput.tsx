import { useEffect, useRef, useState, type ReactNode } from 'react';

// Предупреждения жёлтым; ошибки (error, *Error:, file:line:col:) красным.
function lineTone(line: string) {
  if (/\bwarning\b/i.test(line)) return 'text-amber-300';
  if (/\berror\b|^\s*\w*Error\b|^\S+:\d+(:\d+)?:/i.test(line)) return 'text-red-300';
  return undefined;
}

interface TerminalOutputProps {
  text: string;
  /** Подпись над выводом: «stderr:», «$ gcc main.c». */
  label?: string;
  /** Ограничение высоты, дальше прокрутка. */
  maxHeight?: string;
  /** Переносить длинные строки (ошибки матчей). Лог компилятора не переносится:
   *  стрелки ^^^ под строкой должны указывать на свою колонку. */
  wrap?: boolean;
  /** Кнопка «копировать»; в длинных списках её лучше убрать. */
  copyable?: boolean;
  /** Свои кнопки в шапке перед «копировать» (скачать). */
  actions?: ReactNode;
  /** Своя раскраска строки вместо тонов лога (код шаблона). */
  renderLine?: (line: string) => ReactNode;
}

// Логи компилятора и ошибки матчей как в терминале: моноширинный шрифт,
// переносы сохраняются, длинный вывод прокручивается, есть копирование.
export function TerminalOutput({ text, label, maxHeight = 'max-h-64', wrap = false, copyable = true, actions, renderLine }: TerminalOutputProps) {
  const [copied, setCopied] = useState(false);
  const [scrollable, setScrollable] = useState(false);
  const preRef = useRef<HTMLPreElement>(null);
  const canCopy = copyable && typeof navigator !== 'undefined' && !!navigator.clipboard;

  // в фокус с клавиатуры попадает только вывод, который есть что прокручивать;
  // group, а не region: одинаковые подписи «stderr:» дали бы неразличимые ориентиры
  useEffect(() => {
    const pre = preRef.current;
    if (!pre) return;
    const check = () =>
      setScrollable(pre.scrollHeight > pre.clientHeight || pre.scrollWidth > pre.clientWidth);
    check();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(check);
    observer.observe(pre);
    return () => observer.disconnect();
  }, [text, wrap, maxHeight]);

  const copy = () => {
    navigator.clipboard.writeText(text).then(
      () => {
        setCopied(true);
        setTimeout(() => setCopied(false), 1500);
      },
      () => {},
    );
  };

  return (
    <div className="rounded border border-gray-800 bg-gray-950 font-mono text-xs">
      {(label || canCopy || actions) && (
        <div className="flex items-center justify-between gap-2 px-3 py-1 border-b border-gray-800 text-gray-500">
          <span className="truncate">{label}</span>
          <span className="flex shrink-0 gap-2">
            {actions}
            {canCopy && (
              <button type="button" onClick={copy} aria-live="polite" className="btn btn-sm btn-secondary">
                {copied ? 'скопировано' : 'копировать'}
              </button>
            )}
          </span>
        </div>
      )}
      <pre
        ref={preRef}
        tabIndex={scrollable ? 0 : undefined}
        role={scrollable ? 'group' : undefined}
        aria-label={scrollable ? label || 'вывод' : undefined}
        className={`${maxHeight} overflow-auto ${wrap ? 'whitespace-pre-wrap wrap-anywhere' : 'whitespace-pre'} p-3 leading-relaxed text-gray-300`}
      >
        {text.split('\n').map((line, i) => (
          <span key={i} className={renderLine ? undefined : lineTone(line)}>
            {renderLine ? renderLine(line) : line}
            {'\n'}
          </span>
        ))}
      </pre>
    </div>
  );
}
