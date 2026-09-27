import { useId } from 'react';
import { Markdown, ScrollX } from '../ui/Markdown';
import { LANGUAGES } from '../../utils/starters';

// Схема обмена строками: программа слева, как «←/→» в примерах и шаблонах. Ввод от судьи
// (stdin) голубым, ход программы (stdout) фиолетовым.
const STDIN = 'text-cyan-300';
const STDOUT = 'text-primary-300';

// «Анатомия хода»: общий для всех игр разбор протокола, четыре правила и сброс
// вывода по языкам. Главная ловушка новичка - ход, застрявший в буфере.
function ProtocolAnatomy() {
  const id = useId();
  return (
    <section aria-labelledby={id} className="md my-8 rounded border border-gray-800 bg-gray-900/40 p-4">
      <h2 id={id}>Анатомия хода</h2>
      <p>
        Программа запускается один раз на матч и общается с судьёй строками: судья пишет в её <code>stdin</code>,
        программа отвечает в <code>stdout</code>.
      </p>
      <pre
        role="img"
        aria-label="Схема: судья присылает программе параметры игры, программа отвечает ходом, судья присылает ход соперника, и так каждую итерацию"
        className="w-fit max-w-full overflow-x-auto"
      >
        {' программа               судья\n'}
        {'     │'}<span className={STDIN}>◀── параметры игры ──</span>{'│\n'}
        {'     │'}<span className={STDOUT}>────── ваш ход ─────▶</span>{'│\n'}
        {'     │'}<span className={STDIN}>◀── ход соперника ───</span>{'│\n'}
        {'     │          …          │\n'}
        {'     ╵                     ╵'}
      </pre>
      <ol>
        <li>
          <strong>Старт.</strong> Судья присылает параметры игры, по одному числу в строке: сколько их и в каком порядке, сказано в
          «Протоколе» ниже.
        </li>
        <li>
          <strong>Итерация.</strong> Программа выводит ход одной строкой, сбрасывает вывод и читает ход соперника. Итераций столько,
          сколько пришло на старте. В аукционе иначе: сначала приходит ставка соперника, потом ваша; число со старта — максимум,
          торги кончаются на пасе, а о пасе соперника судья не сообщает.
        </li>
        <li>
          <strong>Конец.</strong> Ввод заканчивается, судья завершает программу. Отдельного сообщения о конце нет.
        </li>
      </ol>

      <h3>Четыре правила</h3>
      <ol>
        <li>Одна строка — один ход: только число или слово, без подписей.</li>
        <li>
          Регистр важен: <code>COOPERATE</code>, а не <code>cooperate</code>.
        </li>
        <li>На ответ 200 мс, первый ход — вместе с запуском программы. Опоздание — поражение в матче.</li>
        <li>
          Отладка — только в <code>stderr</code> и умеренно: <code>stdout</code> читает судья. Если программа упадёт, последние 2 КБ
          stderr попадут в текст ошибки матча.
        </li>
      </ol>

      <h3>Сброс вывода</h3>
      <p>Без сброса ход остаётся в буфере программы: судья его не получает и через 200 мс засчитывает поражение.</p>
      <ScrollX as="div" label="Сброс вывода, прокрутка по горизонтали">
        <table>
          <thead>
            <tr>
              <th>Язык</th>
              <th>После каждой строки</th>
            </tr>
          </thead>
          <tbody>
            {LANGUAGES.map((lang) => (
              <tr key={lang.id}>
                <td>{lang.label}</td>
                <td>
                  <code>{lang.flush}</code>
                  {lang.flushNote && <span className="text-gray-400"> — {lang.flushNote}</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </ScrollX>
    </section>
  );
}

// Правила игры: «Анатомия хода» встаёт перед разделом «Протокол», без него - в конце.
// Первый заголовок «# Название» повторяет заголовок страницы и не выводится.
export function GameRules({ rules }: { rules: string }) {
  const body = rules.replace(/^#\s[^\n]*\n*/, '');
  const at = body.search(/^##\s+Протокол/m);
  const head = at < 0 ? body : body.slice(0, at);
  const tail = at < 0 ? '' : body.slice(at);
  return (
    <>
      <Markdown>{head}</Markdown>
      <ProtocolAnatomy />
      {tail && <Markdown>{tail}</Markdown>}
    </>
  );
}
