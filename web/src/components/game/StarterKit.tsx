import { useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { Tabs } from '../ui/Tabs';
import { Spinner } from '../ui/Spinner';
import { ErrorState } from '../ui/ErrorState';
import { TerminalOutput } from '../ui/TerminalOutput';
import { ioTone } from '../../utils/markdown';
import { saveFile } from '../../utils/saveFile';
import { LANGUAGES, starterLoader, type Language } from '../../utils/starters';

const LANG_KEY = 'starter_lang';

// выбранный язык общий для всех игр и переживает перезагрузку
function useStoredLang(): [string, (id: string) => void] {
  const [lang, setLang] = useState(() => {
    try {
      return localStorage.getItem(LANG_KEY) ?? LANGUAGES[0].id;
    } catch {
      return LANGUAGES[0].id;
    }
  });
  const choose = (id: string) => {
    setLang(id);
    try {
      localStorage.setItem(LANG_KEY, id);
    } catch {
      // хранилище недоступно (приватный режим): язык живёт до перезагрузки
    }
  };
  return [lang, choose];
}

// Строка кода: комментарий приглушён, а подписи протокола «← …» и «→ …» окрашены как в «Анатомии хода».
function codeLine(marker: string) {
  return (line: string) => {
    const at = line.indexOf(marker);
    if (at < 0) return line;
    const comment = line.slice(at);
    return (
      <>
        {line.slice(0, at)}
        <span className={ioTone(comment.slice(marker.length)) ?? 'text-gray-500'}>{comment}</span>
      </>
    );
  };
}

// в скрипте комментарий - только целая строка: # встречается и внутри команд
const Heading = ({ id, children }: { id: string; children: string }) => (
  <h2 id={id} className="font-mono text-base font-semibold text-gray-100">
    <span aria-hidden="true" className="text-primary-400">$ </span>
    {children}
  </h2>
);

// Блок «$ начать»: шаблон бота на выбранном языке (копировать, скачать).
// У игры без шаблонов ничего не выводит.
// id заголовка «starter» - цель перехода из карточки программы.
export function StarterKit({ game }: { game: string }) {
  const [langId, setLangId] = useStoredLang();
  const langs = LANGUAGES.filter((l) => starterLoader(game, l));
  const lang: Language | undefined = langs.find((l) => l.id === langId) ?? langs[0];
  const load = lang && starterLoader(game, lang);
  // код вместе с языком: пока грузится новый язык, на месте остаётся прежний
  // шаблон целиком (подпись, комментарии, скачивание), а блок не схлопывается в спиннер
  const query = useQuery({
    queryKey: ['starter', game, lang?.id ?? ''],
    queryFn: async () => ({ lang: lang!, code: await load!() }),
    enabled: !!load,
    staleTime: Infinity,
    placeholderData: keepPreviousData,
  });

  if (!lang) return null;
  const shown = query.data;

  return (
    <div className="card space-y-8">
      <section aria-labelledby="starter">
        <Heading id="starter">начать</Heading>
        <p className="mt-1 mb-4 text-sm text-gray-400">
          Шаблон программы: читает параметры игры, делает простой ход и сбрасывает вывод. Каждая строка протокола подписана
          комментарием — замените стратегию на свою.
        </p>
        <Tabs
          label="Язык шаблона"
          items={langs.map((l) => ({ id: l.id, label: l.label }))}
          active={lang.id}
          onChange={setLangId}
        >
          {shown ? (
            <div aria-busy={query.isPlaceholderData}>
              <TerminalOutput
                text={shown.code.trimEnd()}
                label={`${shown.lang.file} · ${shown.lang.version}`}
                maxHeight="max-h-[32rem]"
                renderLine={codeLine(shown.lang.comment)}
                actions={
                  <button
                    type="button"
                    onClick={() => saveFile(new Blob([shown.code], { type: 'text/plain' }), shown.lang.file)}
                    className="btn btn-sm btn-secondary"
                  >
                    скачать {shown.lang.file}
                  </button>
                }
              />
            </div>
          ) : query.isError ? (
            <ErrorState message="Не удалось загрузить шаблон" onRetry={() => void query.refetch()} />
          ) : (
            <div className="py-8 text-center text-sm text-gray-400">
              <Spinner>загрузка шаблона</Spinner>
            </div>
          )}
        </Tabs>
      </section>

    </div>
  );
}
