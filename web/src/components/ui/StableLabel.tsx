// Подпись кнопки, которая меняется по состоянию («Запуск...»), без смены ширины:
// все варианты лежат в одной ячейке сетки, виден один. Ширина - по самому
// длинному, поэтому соседние кнопки в ряду не сдвигаются. Скрытые варианты
// aria-hidden: без CSS (jsdom в тестах) они вошли бы в имя кнопки.
export function StableLabel<T extends string>({ options, value }: { options: readonly T[]; value: NoInfer<T> }) {
  return (
    <span className="inline-grid justify-items-center">
      {options.map((o) => (
        <span key={o} aria-hidden={o === value ? undefined : true} className={`col-start-1 row-start-1 ${o === value ? '' : 'invisible'}`}>
          {o}
        </span>
      ))}
    </span>
  );
}
