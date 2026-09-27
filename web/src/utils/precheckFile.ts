// Предпроверка файла программы в браузере, до отправки. error - файл не примут
// или он заведомо не соберётся; warnings - частые причины таймаута и падений,
// загрузку они не блокируют.

// Как detectLanguage в internal/handlers/program.go
export const SUPPORTED_EXTENSIONS = ['.py', '.cpp', '.cc', '.cxx', '.c', '.go', '.rs', '.java', '.js', '.rb', '.php', '.lua'];
// maxFileSize в internal/handlers/program.go
export const MAX_FILE_SIZE = 10 * 1024 * 1024;

export interface Precheck {
  error?: string;
  warnings: string[];
}

export function extensionOf(name: string) {
  const dot = name.lastIndexOf('.');
  return dot < 0 ? '' : name.slice(dot).toLowerCase();
}

const FAMILY: Record<string, string> = {
  '.py': 'Python', '.c': 'C/C++', '.cpp': 'C/C++', '.cc': 'C/C++', '.cxx': 'C/C++', '.go': 'Go',
  '.rs': 'Rust', '.java': 'Java', '.js': 'JavaScript', '.rb': 'Ruby', '.php': 'PHP', '.lua': 'Lua',
};

// Явные признаки чужого языка: по ним видно, что расширение перепутано.
const LOOKS_LIKE: [string, RegExp][] = [
  ['C/C++', /^\s*#include\s*[<"]/m],
  ['Python', /^\s*def \w+\(.*\)\s*:|^\s*import sys\b|__name__\s*==\s*['"]__main__['"]/m],
  ['Rust', /\bfn main\s*\(/],
  ['Java', /\bpublic\s+static\s+void\s+main\s*\(/],
  ['Go', /^\s*package main\b/m],
];

const TIMEOUT = 'без сброса ход застрянет в буфере и программа не ответит вовремя';

// Правила по расширению: [расширения, условие, предупреждение].
const RULES: [string[], (src: string) => string | false][] = [
  [['.c'], (s) => /#include\s*<(iostream|vector|string|algorithm|map|bits\/stdc\+\+\.h)>|\bstd::|using namespace std/.test(s) && 'похоже на C++, а расширение .c: переименуйте файл в .cpp'],
  [['.c'], (s) => /\b(printf|puts|putchar|fputs|fwrite)\s*\(/.test(s) && !/fflush|setvbuf|setbuf/.test(s) && `нет fflush(stdout): ${TIMEOUT}`],
  [['.cpp', '.cc', '.cxx'], (s) => /\bcout\b|\bprintf\s*\(|\bputs\s*\(/.test(s) && !/fflush|endl|flush|setvbuf|unitbuf/.test(s) && `нет std::endl, std::flush или fflush(stdout): ${TIMEOUT}`],
  [['.py'], (s) => /sys\.stdin/.test(s) && /\bprint\s*\(|sys\.stdout\.write/.test(s) && !/flush\s*=\s*True|\.flush\(\)|line_buffering/.test(s) && `ввод через sys.stdin, а вывод не сбрасывается: добавьте print(..., flush=True), ${TIMEOUT}`],
  [['.py'], (s) => { const m = s.match(/^\s*(?:import|from)\s+(numpy|pandas|scipy|sklearn|torch|requests)\b/m); return !!m && `библиотеки ${m[1]} нет: доступна только стандартная библиотека Python`; }],
  [['.rb'], (s) => /\b(puts|print)\b/.test(s) && !/sync\s*=\s*true|\.flush/.test(s) && `нет STDOUT.sync = true: ${TIMEOUT}`],
  [['.lua'], (s) => /io\.write|\bprint\s*\(/.test(s) && !/flush|setvbuf/.test(s) && `нет io.stdout:flush(): ${TIMEOUT}`],
  [['.go'], (s) => /bufio\.NewWriter\(os\.Stdout\)/.test(s) && !/\.Flush\(\)/.test(s) && `bufio.Writer без Flush(): ${TIMEOUT}`],
  [['.go'], (s) => { const m = s.match(/"([\w-]+\.[\w.-]+\/[^"]+)"/); return !!m && `модуля ${m[1]} нет: доступна только стандартная библиотека Go`; }],
  [['.rs'], (s) => /\buse\s+rand\b|\bextern\s+crate\s+rand\b/.test(s) && 'крейта rand нет: сборка идёт через rustc без cargo, только std'],
  [['.java'], (s) => !/\bclass\s+\w+/.test(s) && 'не найдено объявление class: без него Java-программа не соберётся'],
];

/** Проверка содержимого: ext - расширение с точкой, bytes - файл целиком. */
export function precheckSource(ext: string, bytes: Uint8Array): Precheck {
  if (bytes.length === 0) return { error: 'файл пустой', warnings: [] };
  if (bytes.subarray(0, 8000).includes(0)) {
    return { error: 'похоже на бинарный файл: загрузите исходный код, а не собранную программу', warnings: [] };
  }

  const warnings: string[] = [];
  try {
    new TextDecoder('utf-8', { fatal: true }).decode(bytes);
  } catch {
    warnings.push('файл не в UTF-8 (например, cp1251): русские буквы в комментариях и строках могут сломать сборку, сохраните его в UTF-8');
  }
  const src = new TextDecoder().decode(bytes);

  const other = LOOKS_LIKE.find(([family, re]) => family !== FAMILY[ext] && re.test(src));
  if (other) warnings.push(`похоже на ${other[0]}, а расширение ${ext}: собираться файл будет как ${FAMILY[ext]}`);

  for (const [exts, check] of RULES) {
    const w = exts.includes(ext) && check(src);
    if (w) warnings.push(w);
  }
  return { warnings };
}

export async function precheckFile(file: File): Promise<Precheck> {
  const ext = extensionOf(file.name);
  if (!SUPPORTED_EXTENSIONS.includes(ext)) {
    return { error: `формат ${ext || 'без расширения'} не поддерживается, подойдут: ${SUPPORTED_EXTENSIONS.join(', ')}`, warnings: [] };
  }
  if (file.size > MAX_FILE_SIZE) return { error: 'файл больше 10 МБ', warnings: [] };
  return precheckSource(ext, new Uint8Array(await file.arrayBuffer()));
}
