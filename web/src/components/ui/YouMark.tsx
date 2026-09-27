// Метка своей строки или стороны: «> вы». Знак грамматики только для глаз.
// Не сжимается: рядом с длинным названием обрезается название, а не метка.
export function YouMark({ className = 'ml-2 text-xs' }: { className?: string }) {
  return (
    <span className={`shrink-0 font-mono font-normal text-primary-400 whitespace-nowrap ${className}`}>
      <span aria-hidden="true">&gt; </span>вы
    </span>
  );
}
