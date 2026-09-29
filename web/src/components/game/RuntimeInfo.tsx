import { LANGUAGES } from '../../utils/starters';

// Раскрывашка «Среда исполнения»: версии языков в песочнице и лимиты матча
// (docs/USER_GUIDE.md, internal/executor). Под зоной загрузки и на странице игры.
export function RuntimeInfo() {
  return (
    <details>
      <summary className="w-fit cursor-pointer font-mono text-xs text-gray-400 hover:text-gray-200">среда исполнения</summary>
      <div className="md mt-3 text-xs">
        <table className="text-xs">
          <thead>
            <tr>
              <th>Язык</th>
              <th>Версия и сборка</th>
            </tr>
          </thead>
          <tbody>
            {LANGUAGES.map((lang) => (
              <tr key={lang.id}>
                <td>{lang.label}</td>
                <td>{lang.version}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <ul>
          <li>Только стандартная библиотека: pip, npm, cargo и сторонних модулей Go нет, сети тоже.</li>
          <li>Исходник до 10 МБ, собранная программа до 32 МБ. Интерпретируемые языки проверяются на синтаксис.</li>
          <li>На ход 200 мс, на первый с запуском программы до 2 с. На матч 60 с, одно ядро и 512 МБ памяти на обе программы.</li>
          <li>У программы до 48 процессов и потоков, файлы до 4 МБ.</li>
        </ul>
      </div>
    </details>
  );
}
