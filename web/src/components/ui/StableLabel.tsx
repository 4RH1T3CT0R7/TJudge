// Подпись кнопки, которая меняется по состоянию («Запуск...»), без смены ширины:
// все варианты лежат в одной ячейке сетки, виден один. Ширина - по самому
// длинному, поэтому соседние кнопки в ряду не сдвигаются.
export function StableLabel({ options, value }: { options: string[]; value: string }) {
  return (
    <span className="inline-grid justify-items-center">
      {options.map((o) => (
        <span key={o} className={`col-start-1 row-start-1 ${o === value ? '' : 'invisible'}`}>
          {o}
        </span>
      ))}
    </span>
  );
}
