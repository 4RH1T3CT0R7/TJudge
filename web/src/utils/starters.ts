// Языки программ и шаблоны ботов (src/starters/<игра>/<язык>/<файл>).
// Версии и команды сборки - из образа docker/builder и internal/executor/compiler.go:
// при смене образа таблицу нужно обновить.

export interface Language {
  /** Язык программы в API и каталог шаблона. */
  id: string;
  label: string;
  /** Имя шаблона и скачанного файла. */
  file: string;
  /** Версия в песочнице и сборка на сервере. */
  version: string;
  /** Как сбросить вывод после строки. */
  flush: string;
  flushNote?: string;
  /** Начало однострочного комментария: комментарии в коде шаблона приглушены. */
  comment: string;
  /** Сборка для tjudge-cli у себя; интерпретируемым хватает chmod +x, шебанг есть в шаблоне. */
  build: string;
  /** Что передавать tjudge-cli. */
  run: string;
}

export const LANGUAGES: Language[] = [
  {
    id: 'python', label: 'Python', file: 'main.py', version: 'Python 3.12', comment: '#',
    flush: 'print(x, flush=True)', flushNote: 'input() сбрасывает вывод сам, чтение из sys.stdin — нет',
    build: 'chmod +x main.py', run: './main.py',
  },
  {
    id: 'cpp', label: 'C++', file: 'main.cpp', version: 'GCC 14.2, g++ -O2', comment: '//',
    flush: 'std::cout << x << std::endl;',
    build: 'g++ -O2 -o main main.cpp', run: './main',
  },
  {
    id: 'c', label: 'C', file: 'main.c', version: 'GCC 14.2, gcc -O2 -lm', comment: '//',
    flush: 'printf(...); fflush(stdout);',
    build: 'gcc -O2 -o main main.c -lm', run: './main',
  },
  {
    id: 'java', label: 'Java', file: 'Main.java', version: 'OpenJDK 17', comment: '//',
    flush: 'System.out.println(x); System.out.flush();',
    // tjudge-cli запускает исполняемый файл: для Java нужна обёртка
    build: `javac Main.java && printf '#!/bin/sh\\nexec java -cp "%s" Main\\n' "$PWD" > main && chmod +x main`, run: './main',
  },
  {
    id: 'go', label: 'Go', file: 'main.go', version: 'Go 1.24, go build', comment: '//',
    flush: 'fmt.Println(x)', flushNote: 'os.Stdout без буфера; с bufio.Writer нужен w.Flush()',
    build: 'go build -o main main.go', run: './main',
  },
  {
    id: 'rust', label: 'Rust', file: 'main.rs', version: 'rustc 1.87 -O, без cargo', comment: '//',
    flush: 'println!(...); io::stdout().flush().unwrap();',
    build: 'rustc -O -o main main.rs', run: './main',
  },
  {
    id: 'javascript', label: 'JavaScript', file: 'main.js', version: 'Node.js 22', comment: '//',
    flush: 'console.log(x)', flushNote: 'сбрасывает сам',
    build: 'chmod +x main.js', run: './main.js',
  },
  {
    id: 'ruby', label: 'Ruby', file: 'main.rb', version: 'Ruby 3.4', comment: '#',
    flush: 'STDOUT.sync = true', flushNote: 'один раз в начале программы',
    build: 'chmod +x main.rb', run: './main.rb',
  },
  {
    id: 'php', label: 'PHP', file: 'main.php', version: 'PHP 8.3', comment: '//',
    flush: 'echo $x, "\\n"; flush();',
    build: 'chmod +x main.php', run: './main.php',
  },
  {
    id: 'lua', label: 'Lua', file: 'main.lua', version: 'Lua 5.4', comment: '--',
    flush: 'io.write(x, "\\n"); io.stdout:flush()',
    build: 'chmod +x main.lua', run: './main.lua',
  },
];

// Коммит tjudge-cli в образе матчей (TJUDGE_CLI_REF в docker/tjudge/Dockerfile):
// у себя участник ставит тот же, чтобы правила и подсчёт очков совпадали.
export const TJUDGE_CLI_REV = '600879b5e81f86cdd06b877227e2c1ef9e6dca1c';

// шаблоны грузятся отдельными чанками, только когда их открыли
const loaders = import.meta.glob<string>('../starters/*/*/*', { query: '?raw', import: 'default' });

export function starterLoader(game: string, lang: Language): (() => Promise<string>) | undefined {
  return loaders[`../starters/${game}/${lang.id}/${lang.file}`];
}

/** Команды «проверить у себя»: установка tjudge-cli, сборка и матч шаблона с самим собой. */
export function localRunScript(game: string, lang: Language): string {
  return [
    '# судья той же версии, что на сервере (нужен Rust: https://rustup.rs)',
    // перенос строки: иначе хеш коммита уходит за край блока даже на широком экране
    'cargo install --git https://github.com/bmstu-itstech/tjudge-cli \\',
    `  --rev ${TJUDGE_CLI_REV}`,
    '',
    `# сборка ${lang.file}`,
    lang.build,
    '',
    '# матч с самим собой: 20 итераций, -v показывает каждый ход',
    '# tjudge-cli не показывает stderr программы, а после 64 КБ в stderr она зависает:',
    '# отладку у себя пишите в файл, например debug.log',
    `tjudge-cli ${game} -i 20 -v ${lang.run} ${lang.run}`,
  ].join('\n');
}
