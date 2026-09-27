// Метка своей строки или стороны: «> вы». Знак грамматики только для глаз.
export function YouMark() {
  return (
    <span className="ml-2 font-mono text-xs font-normal text-primary-400 whitespace-nowrap">
      <span aria-hidden="true">&gt; </span>вы
    </span>
  );
}
