import { useSearchParams } from 'react-router-dom';

// Активная вкладка хранится в ?tab=: переживает перезагрузку, ссылкой можно поделиться.
// Вкладка по умолчанию в URL не пишется, неизвестное значение означает вкладку по умолчанию.
// replace: переключение вкладок не копится в истории, «Назад» уводит со страницы.
export function useTabParam<T extends string>(ids: readonly T[], fallback: T): [T, (id: T) => void] {
  const [params, setParams] = useSearchParams();
  const raw = params.get('tab');
  const active = ids.find((id) => id === raw) ?? fallback;

  const setActive = (id: T) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (id === fallback) next.delete('tab');
        else next.set('tab', id);
        return next;
      },
      { replace: true },
    );

  return [active, setActive];
}
