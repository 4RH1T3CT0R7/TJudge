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
}

export const LANGUAGES: Language[] = [
  {
    id: 'python', label: 'Python', file: 'main.py', version: 'Python 3.12', comment: '#',
    flush: 'print(x, flush=True)', flushNote: 'input() сбрасывает вывод сам, чтение из sys.stdin — нет',
  },
  {
    id: 'cpp', label: 'C++', file: 'main.cpp', version: 'GCC 14.2, g++ -O2', comment: '//',
    flush: 'std::cout << x << std::endl;',
  },
  {
    id: 'c', label: 'C', file: 'main.c', version: 'GCC 14.2, gcc -O2 -lm', comment: '//',
    flush: 'printf(...); fflush(stdout);',
  },
  {
    id: 'java', label: 'Java', file: 'Main.java', version: 'OpenJDK 17', comment: '//',
    flush: 'System.out.println(x); System.out.flush();',
  },
  {
    id: 'go', label: 'Go', file: 'main.go', version: 'Go 1.24, go build', comment: '//',
    flush: 'fmt.Println(x)', flushNote: 'os.Stdout без буфера; с bufio.Writer нужен w.Flush()',
  },
  {
    id: 'rust', label: 'Rust', file: 'main.rs', version: 'rustc 1.87 -O, без cargo', comment: '//',
    flush: 'println!(...); io::stdout().flush().unwrap();',
  },
  {
    id: 'javascript', label: 'JavaScript', file: 'main.js', version: 'Node.js 22', comment: '//',
    flush: 'console.log(x)', flushNote: 'сбрасывает сам',
  },
  {
    id: 'ruby', label: 'Ruby', file: 'main.rb', version: 'Ruby 3.4', comment: '#',
    flush: 'STDOUT.sync = true', flushNote: 'один раз в начале программы',
  },
  {
    id: 'php', label: 'PHP', file: 'main.php', version: 'PHP 8.3', comment: '//',
    flush: 'echo $x, "\\n"; flush();',
  },
  {
    id: 'lua', label: 'Lua', file: 'main.lua', version: 'Lua 5.4', comment: '--',
    flush: 'io.write(x, "\\n"); io.stdout:flush()',
  },
];

// шаблоны грузятся отдельными чанками, только когда их открыли
const loaders = import.meta.glob<string>('../starters/*/*/*', { query: '?raw', import: 'default' });

export function starterLoader(game: string, lang: Language): (() => Promise<string>) | undefined {
  return loaders[`../starters/${game}/${lang.id}/${lang.file}`];
}
