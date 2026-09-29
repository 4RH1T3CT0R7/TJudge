import type { Match } from '../types';

// Вердикты по ошибкам программ по-русски. Текст ошибки матча собирают
// executor.parseResult и docker/tjudge/sandbox.sh:
//   Программа 1 завершилась с ошибкой:
//   --- stderr ---
//   left player error: <причина от tjudge-cli>
//   --- stderr программы (последние 2 КБ) ---
//   <хвост stderr упавшей программы>

export type Side = 1 | 2;

export interface Explanation {
  /** Вердикт одной фразой, от лица «вы», если упала своя программа. */
  verdict: string;
  /** Что проверить. */
  hint?: string;
}

const TAIL_MARK = '--- stderr программы (последние 2 КБ) ---';
const OVERFLOW_MARK = '--- stderr больше 4 МБ';

const FLUSH_HINT =
  'сбрасывайте вывод после каждого хода: fflush(stdout), print(..., flush=True), STDOUT.sync = true, io.flush(); время на ответ включает запуск';
const STDOUT_HINT = 'в stdout — только ход, одной строкой; отладку печатайте в stderr';
const NUMBER_HINT = 'ход — одно целое число без знака, дробной части и лишних символов, одной строкой';

interface Reason {
  phrase: string;
  hint?: string;
}

// Причины из tjudge-cli (games/*.rs, subprocess_player.rs): фраза без подлежащего.
const REASONS: [RegExp, (m: RegExpMatchArray) => Reason][] = [
  [/timed out/i, () => ({ phrase: 'не ответила вовремя (200 мс на ход, 2 с на первый)', hint: FLUSH_HINT })],
  [
    /terminated unexpectedly|broken pipe/i,
    () => ({ phrase: 'завершилась посреди матча', hint: 'она упала или вышла из цикла раньше конца игры: читайте ходы, пока идёт ввод' }),
  ],
  [
    /unknown action '(.*)', expected one of \[(.*)\]/,
    (m) => {
      const expected = [...m[2].matchAll(/'([^']*)'/g)].map((x) => x[1]);
      const caseOnly = expected.includes(m[1].trim().toUpperCase());
      return {
        phrase: `ответила «${m[1]}» вместо ${expected.join(' или ')}`,
        hint: caseOnly ? 'регистр важен: ход пишется заглавными буквами' : STDOUT_HINT,
      };
    },
  ],
  [/expected claim in \[(-?\d+), (-?\d+)\], got (-?\d+)/, (m) => ({ phrase: `заявила ${m[3]}, а можно от ${m[1]} до ${m[2]}` })],
  [/expected spent <= energy, got (-?\d+) > (-?\d+)/, (m) => ({ phrase: `потратила ${m[1]} при остатке энергии ${m[2]}` })],
  [/expected contribution <= endowment, got (\S+) > (\S+)/, (m) => ({ phrase: `вложила ${m[1]} при запасе ${m[2]}` })],
  [/bid must be non-negative, got (-?\d+)/, (m) => ({ phrase: `поставила отрицательную ставку ${m[1]}` })],
  [
    /bid must be > opponent's last bid \((-?\d+)\), got (-?\d+)/,
    (m) => ({ phrase: `поставила ${m[2]}, а нужно больше ставки соперника ${m[1]}`, hint: 'чтобы выйти из торгов, ставьте 0' }),
  ],
  // ходы всех игр - целые неотрицательные числа (u32 или i32), разбор через parse() Rust
  [/empty string/, () => ({ phrase: 'ответила пустой строкой', hint: STDOUT_HINT })],
  [/number too (large|small)/, (m) => ({ phrase: `ответила слишком ${m[1] === 'large' ? 'большим' : 'маленьким'} числом`, hint: NUMBER_HINT })],
  [/invalid digit/, () => ({ phrase: 'ответила не целым неотрицательным числом', hint: NUMBER_HINT })],
];

/**
 * Сторона, чья программа упала: код выхода tjudge-cli 1 или 2 и победа соперника,
 * как в handlers/match.go redactMatchErrors. Код 1 без победителя - сбой матча
 * в воркере (таймаут матча, мусор в выводе), сторона неизвестна: null.
 */
export function failedSide(match: Pick<Match, 'status' | 'error_code' | 'winner'>): Side | null {
  if (match.status !== 'failed') return null;
  if (match.error_code === 1 && match.winner === 2) return 1;
  if (match.error_code === 2 && match.winner === 1) return 2;
  return null;
}

/** Хвост stderr упавшей программы, который бэкенд приложил к ошибке. */
export function stderrTail(text: string): string | undefined {
  const at = text.indexOf(TAIL_MARK);
  if (at < 0) return undefined;
  const tail = text.slice(at + TAIL_MARK.length).split('\n--- stdout ---')[0];
  return tail.split('\n').filter((l) => !l.startsWith(OVERFLOW_MARK)).join('\n').trim() || undefined;
}

// Последняя строка с ошибкой в хвосте: Traceback, panic, Exception. Кадры стека
// (Java/JS «at …», Ruby «from …») пропускаются: в них тоже бывает «…Exception».
function lastErrorLine(tail: string): string | undefined {
  const lines = tail.split('\n').map((l) => l.trim()).filter(Boolean);
  for (let i = lines.length - 1; i >= 0; i--) {
    if (/^(at|from)\s/.test(lines[i])) continue;
    if (/panicked at/.test(lines[i])) return [lines[i], lines[i + 1]].filter(Boolean).join(' ');
    if (/(Error|Exception)(:|\)?$)|^panic:|Segmentation fault/.test(lines[i])) return lines[i].slice(0, 160);
  }
  return undefined;
}

// Подсказки по хвосту stderr: они точнее общей подсказки причины.
function tailHint(text: string): string | undefined {
  if (text.includes(OVERFLOW_MARK)) return 'stderr больше 4 МБ: печатайте отладку реже';
  const module = text.match(/No module named '([\w.]+)'|Cannot find module '([^']+)'/);
  if (module) return `модуль ${module[1] ?? module[2]} недоступен: только стандартная библиотека`;
  return undefined;
}

/**
 * Вердикт по упавшему матчу. mySide - сторона своей программы (null - зритель),
 * names - названия команд сторон. null - матч не падал.
 */
export function explainMatchError(
  match: Pick<Match, 'status' | 'error_code' | 'error_message' | 'winner'>,
  mySide: Side | null,
  names: [string?, string?] = [],
): Explanation | null {
  if (match.status !== 'failed') return null;
  const side = failedSide(match);
  if (side === null) {
    return {
      // код 1 без победителя - метка сбоя воркера, а не сторона
      verdict: `Матч прерван системой${(match.error_code ?? 0) > 2 ? ` (код ${match.error_code})` : ''}`,
      hint: 'если повторяется, сообщите организатору',
    };
  }
  if (mySide !== null && side !== mySide) {
    return { verdict: 'Программа соперника завершилась с ошибкой: победа ваша' };
  }

  const text = match.error_message ?? '';
  const name = names[side - 1];
  const subject = side === mySide ? 'Ваша программа' : name ? `Программа команды «${name}»` : `Программа ${side}`;
  const reasonLine = text.match(/(?:left|right) player error: (.*)/)?.[1] ?? '';
  let reason: Reason = { phrase: 'завершилась с ошибкой' };
  for (const [re, make] of REASONS) {
    const m = reasonLine.match(re);
    if (m) {
      reason = make(m);
      break;
    }
  }
  const tail = stderrTail(text);
  const errLine = tail && lastErrorLine(tail);
  return {
    verdict: `${subject} ${reason.phrase}${errLine ? `: ${errLine}` : ''}`,
    hint: tailHint(text) ?? reason.hint,
  };
}

// Заголовки C, где объявлены частые функции: «implicit declaration» лечится #include.
const C_HEADERS: Record<string, string> = {
  rand: 'stdlib.h', srand: 'stdlib.h', malloc: 'stdlib.h', free: 'stdlib.h', exit: 'stdlib.h', atoi: 'stdlib.h', abs: 'stdlib.h',
  printf: 'stdio.h', scanf: 'stdio.h', puts: 'stdio.h', fflush: 'stdio.h', getchar: 'stdio.h', fgets: 'stdio.h',
  strlen: 'string.h', strcmp: 'string.h', strcpy: 'string.h', memset: 'string.h',
  sqrt: 'math.h', pow: 'math.h', fabs: 'math.h', time: 'time.h',
};

const COMPILE_HINTS: [RegExp, (m: RegExpMatchArray) => string][] = [
  [
    /implicit declaration of function '(\w+)'/,
    (m) => `функция ${m[1]} не объявлена: подключите ${C_HEADERS[m[1]] ? `#include <${C_HEADERS[m[1]]}>` : 'нужный заголовок'}`,
  ],
  [/fatal error: ([\w/.+-]+): No such file or directory/, (m) => `заголовка ${m[1]} нет: доступна только стандартная библиотека`],
  [/undefined reference to `main'/, () => 'не найдена функция main'],
  [
    /unresolved import `(\w+)|can't find crate for `(\w+)`/,
    (m) => `крейт ${m[1] ?? m[2]} недоступен: сборка идёт через rustc без cargo, только std`,
  ],
  [/is not in std|cannot find package|no required module provides package/, () => 'внешние модули Go недоступны, только стандартная библиотека'],
  [/declared and not used|imported and not used/, () => 'Go не собирает код с неиспользуемыми переменными и импортами: удалите их'],
  [/error\[E0308\]: mismatched types/, () => 'типы не совпадают: приведите явно (as u32, try_into)'],
  [/\(mismatched types (\w+) and (\w+)\)/, (m) => `типы ${m[1]} и ${m[2]} не смешиваются: приведите явно, например ${m[2]}(x)`],
  [/Missing parentheses in call to 'print'/, () => 'это синтаксис Python 2, а сборка идёт в Python 3.12: print(...)'],
  // py_compile: «IndentationError: unexpected indent (main.py, line 5)»
  [/TabError/, () => 'в отступах смешаны табы и пробелы: оставьте что-то одно'],
  [
    /IndentationError: (unexpected indent|expected an indented block|unindent)[^\n]*, line (\d+)\)/,
    (m) =>
      m[1] === 'unexpected indent'
        ? `лишний отступ в строке ${m[2]}`
        : m[1] === 'unindent'
          ? `отступ в строке ${m[2]} не совпадает ни с одним уровнем выше`
          : `в строке ${m[2]} не хватает отступа: тело if, for, def сдвигается вправо`,
  ],
  [/IndentationError/, () => 'ошибка в отступах: смотрите строку в выводе ниже'],
  [/не найдено объявление class/, () => 'объявите class с методом public static void main(String[] args)'],
  [/cannot find symbol|was not declared in this scope/, () => 'имя не найдено: проверьте опечатки и подключённые заголовки или импорты'],
  [/invalid UTF-8|stream did not contain valid UTF-8|Non-UTF-8 code/i, () => 'сохраните файл в кодировке UTF-8'],
];

/** Подсказка к логу компилятора; undefined - частой ошибки не узнали. */
export function explainCompileError(log: string): string | undefined {
  for (const [re, make] of COMPILE_HINTS) {
    const m = log.match(re);
    if (m) return make(m);
  }
  return undefined;
}
