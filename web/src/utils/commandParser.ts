// Python-style command parser for the interactive terminal quest.
// Tokenizes commands like `invader.jump(3)` and returns structured actions.

import type { InvaderPose } from '../components/SpaceInvader';

// --- Types ---

export type InvaderAction = {
  type: 'invader';
  pose?: InvaderPose;
  jump?: number;
  spin?: number;
  say?: string;
  transform?: 'fire' | 'ice' | 'ghost' | 'rainbow' | null;
  mood?: 'happy' | 'neutral' | 'scared' | 'angry' | 'sleepy';
};

export type TerminalAction = {
  type: 'terminal';
  action: 'clear' | 'help' | 'history';
};

export type GameAction = {
  type: 'game';
  game: 'pong' | 'typing' | 'strategy';
};

export type QuestAction = {
  type: 'quest';
  action: 'start' | 'scan' | 'decrypt' | 'hack' | 'debug' | 'patch' | 'escape' | 'inventory';
  args?: string[];
};

export type CommandAction = InvaderAction | TerminalAction | GameAction | QuestAction;

export interface CommandResult {
  output: string[];
  color?: string;
  action?: CommandAction;
  sound?: 'click' | 'success' | 'error' | 'levelUp' | 'escape';
}

// --- Virtual filesystem ---
const VIRTUAL_FS: Record<string, string> = {
  'readme.txt': '// TJudge System v3.7\n// Инвейдер сбежал из кода.\n// Используйте scan() для поиска следов.',
  'config.sys': 'FIREWALL=active\nPORT=8080\nSECRET_KEY=***encrypted***',
  'logs/error.log': '[WARN] entity "invader" escaped sandbox\n[ERR] containment breach at sector 7',
  'logs/access.log': '192.168.1.1 - GET /api/escape - 403\n10.0.0.42 - POST /api/hack - 401',
  '.hidden': '// Секретный файл!\n// Код доступа: NASH-1950',
  'games/pong.exe': '>> Запустите: play.pong()',
  'games/strategy.dat': '>> Данные стратегий. Запустите: play.strategy()',
};

// --- All known commands for tab completion ---
export const ALL_COMMANDS = [
  'invader.jump()', 'invader.spin(360)', 'invader.say("")', 'invader.dance()',
  'invader.fly()', 'invader.attack()', 'invader.shield()', 'invader.sleep()',
  'invader.cry()', 'invader.teleport()', 'invader.transform("")', 'invader.pose()',
  'invader.status()',
  'help', 'clear', 'whoami', 'ls', 'history',
  'cat readme.txt', 'cat config.sys', 'cat .hidden',
  'ping tjudge.io',
  'scan()', 'decrypt("")', 'hack("")', 'debug()', 'patch()', 'escape()',
  'play.pong()', 'play.typing()', 'play.strategy()',
  'inventory()', 'quest.start()', 'start',
  'nash()', 'stackoverflow()',
];

// --- Parser ---

interface ParsedCommand {
  object: string;
  method: string;
  args: (string | number)[];
}

function parseCommand(input: string): ParsedCommand | null {
  const trimmed = input.trim();
  if (!trimmed) return null;

  // `object.method(args)` pattern
  const dotCall = trimmed.match(/^(\w+)\.(\w+)\(([^)]*)\)$/);
  if (dotCall) {
    const [, obj, method, rawArgs] = dotCall;
    const args = rawArgs
      ? rawArgs.split(',').map((a) => {
          const s = a.trim();
          // String arg
          if ((s.startsWith('"') && s.endsWith('"')) || (s.startsWith("'") && s.endsWith("'"))) {
            return s.slice(1, -1);
          }
          const n = Number(s);
          return isNaN(n) ? s : n;
        })
      : [];
    return { object: obj, method, args };
  }

  // `function(args)` pattern
  const funcCall = trimmed.match(/^(\w+)\(([^)]*)\)$/);
  if (funcCall) {
    const [, func, rawArgs] = funcCall;
    const args = rawArgs
      ? rawArgs.split(',').map((a) => {
          const s = a.trim();
          if ((s.startsWith('"') && s.endsWith('"')) || (s.startsWith("'") && s.endsWith("'"))) {
            return s.slice(1, -1);
          }
          const n = Number(s);
          return isNaN(n) ? s : n;
        })
      : [];
    return { object: '_global', method: func, args };
  }

  // Simple commands: `help`, `clear`, `ls`, `whoami`, `start`
  const parts = trimmed.split(/\s+/);
  return { object: '_simple', method: parts[0], args: parts.slice(1) };
}

// --- Executor ---

export function executeCommand(input: string): CommandResult {
  const parsed = parseCommand(input);
  if (!parsed) {
    return { output: [''], color: 'text-gray-500' };
  }

  const { object, method, args } = parsed;

  // --- Easter eggs (check first) ---
  const lower = input.trim().toLowerCase();
  if (lower === 'sudo rm -rf /' || lower === 'sudo rm -rf /*') {
    return {
      output: ['> Хорошая попытка. Инвейдер осуждающе смотрит на вас.'],
      color: 'text-red-400',
      action: { type: 'invader', pose: 'idle', mood: 'angry' },
      sound: 'error',
    };
  }
  if (lower === 'git push --force' || lower === 'git push -f') {
    return {
      output: ['// НЕ В ПРОДЕ!!!'],
      color: 'text-red-500',
      action: { type: 'invader', pose: 'idle', mood: 'scared' },
      sound: 'error',
    };
  }
  if (lower === 'import antigravity') {
    return {
      output: ['>>> Модуль antigravity загружен', '>>> Гравитация отключена...'],
      color: 'text-cyan-400',
      action: { type: 'invader', pose: 'fly', mood: 'happy' },
      sound: 'success',
    };
  }
  if (lower === 'exit' || lower === 'quit') {
    return {
      output: ['> Нельзя выйти из матрицы. Попробуйте escape() на уровне 5.'],
      color: 'text-gray-500',
    };
  }
  if (lower === '42') {
    return {
      output: ['> Ответ на главный вопрос жизни, вселенной и всего такого.'],
      color: 'text-cyan-400',
    };
  }

  // --- Invader commands ---
  if (object === 'invader') {
    switch (method) {
      case 'jump': {
        const n = typeof args[0] === 'number' ? args[0] : 1;
        return {
          output: [`> Инвейдер прыгает${n > 1 ? ` ${n} раз` : ''}!`],
          color: 'text-green-400',
          action: { type: 'invader', jump: n, mood: 'happy' },
          sound: 'success',
        };
      }
      case 'spin': {
        const deg = typeof args[0] === 'number' ? args[0] : 360;
        return {
          output: [`> Инвейдер крутится на ${deg}°!`],
          color: 'text-green-400',
          action: { type: 'invader', spin: deg, pose: 'spin', mood: 'happy' },
          sound: 'success',
        };
      }
      case 'say': {
        const msg = typeof args[0] === 'string' ? args[0] : '...';
        return {
          output: [`> Инвейдер говорит: "${msg}"`],
          color: 'text-purple-400',
          action: { type: 'invader', say: msg },
        };
      }
      case 'dance':
        return {
          output: ['> Инвейдер танцует! ~(^_^)~'],
          color: 'text-green-400',
          action: { type: 'invader', pose: 'dance', mood: 'happy' },
          sound: 'success',
        };
      case 'fly':
        return {
          output: ['> Инвейдер взлетает! Ракетные двигатели активированы.'],
          color: 'text-cyan-400',
          action: { type: 'invader', pose: 'fly', mood: 'happy' },
          sound: 'success',
        };
      case 'attack':
        return {
          output: ['> Инвейдер атакует! >>>----->'],
          color: 'text-red-400',
          action: { type: 'invader', pose: 'attack', mood: 'angry' },
          sound: 'success',
        };
      case 'shield':
        return {
          output: ['> Щит активирован! [====SHIELD====]'],
          color: 'text-green-400',
          action: { type: 'invader', pose: 'shield' },
          sound: 'success',
        };
      case 'sleep':
        return {
          output: ['> Инвейдер засыпает... Zzz'],
          color: 'text-blue-400',
          action: { type: 'invader', pose: 'sleep', mood: 'sleepy' },
        };
      case 'cry':
        return {
          output: ['> Инвейдер плачет... (T_T)'],
          color: 'text-blue-300',
          action: { type: 'invader', pose: 'cry', mood: 'scared' },
        };
      case 'teleport':
        return {
          output: ['> Телепортация...', '> ...перемещение завершено!'],
          color: 'text-purple-400',
          action: { type: 'invader', pose: 'teleport' },
          sound: 'escape',
        };
      case 'transform': {
        const variant = typeof args[0] === 'string' ? args[0] : null;
        const validTransforms: Record<string, 'fire' | 'ice' | 'ghost' | 'rainbow'> = {
          'огонь': 'fire', 'fire': 'fire',
          'лёд': 'ice', 'лед': 'ice', 'ice': 'ice',
          'призрак': 'ghost', 'ghost': 'ghost',
          'радуга': 'rainbow', 'rainbow': 'rainbow',
        };
        const transform = variant ? validTransforms[variant.toLowerCase()] : null;
        if (!transform) {
          return {
            output: ['> Варианты: "огонь", "лёд", "призрак", "радуга"'],
            color: 'text-yellow-400',
            sound: 'error',
          };
        }
        const names: Record<string, string> = {
          fire: 'Огненная форма!', ice: 'Ледяная форма!',
          ghost: 'Призрачная форма!', rainbow: 'Радужная форма!',
        };
        return {
          output: [`> Трансформация: ${names[transform]}`],
          color: 'text-amber-400',
          action: { type: 'invader', pose: 'transform', transform },
          sound: 'success',
        };
      }
      case 'pose':
        return {
          output: [
            '> Доступные позы:',
            '  idle, dance, fly, attack, shield,',
            '  sleep, cry, teleport, transform',
          ],
          color: 'text-gray-400',
        };
      case 'status':
        return {
          output: [
            '> === Статус Инвейдера ===',
            '  Настроение: нейтральное',
            '  Энергия: 100%',
            '  Уровень: МАКС',
          ],
          color: 'text-purple-400',
        };
      default:
        return {
          output: [`> Неизвестная команда: invader.${method}()`, '  Попробуйте help invader'],
          color: 'text-red-400',
          sound: 'error',
        };
    }
  }

  // --- Play commands ---
  if (object === 'play') {
    switch (method) {
      case 'pong':
        return {
          output: ['> Запуск ASCII Pong...'],
          color: 'text-cyan-400',
          action: { type: 'game', game: 'pong' },
          sound: 'success',
        };
      case 'typing':
        return {
          output: ['> Запуск Typing Race...'],
          color: 'text-cyan-400',
          action: { type: 'game', game: 'typing' },
          sound: 'success',
        };
      case 'strategy':
        return {
          output: ['> Запуск Guess Strategy...'],
          color: 'text-cyan-400',
          action: { type: 'game', game: 'strategy' },
          sound: 'success',
        };
      default:
        return {
          output: ['> Игры: play.pong(), play.typing(), play.strategy()'],
          color: 'text-yellow-400',
          sound: 'error',
        };
    }
  }

  // --- Quest commands ---
  if (object === 'quest') {
    if (method === 'start') {
      return {
        output: ['> Запуск квеста...', '> [Уровень 1: Разведка]'],
        color: 'text-purple-400',
        action: { type: 'quest', action: 'start' },
        sound: 'levelUp',
      };
    }
  }

  // --- Global functions ---
  if (object === '_global') {
    switch (method) {
      case 'scan':
        return {
          output: [
            '> Сканирование системы...',
            '  Найдено: 3 файла',
            '  readme.txt, config.sys, .hidden',
            '  Используйте cat <filename> для чтения',
          ],
          color: 'text-cyan-400',
          action: { type: 'quest', action: 'scan' },
          sound: 'success',
        };
      case 'decrypt': {
        const key = typeof args[0] === 'string' ? args[0] : '';
        if (key.toUpperCase() === 'NASH-1950') {
          return {
            output: ['> Расшифровка успешна!', '> Файрвол ослаблен. Можно атаковать.'],
            color: 'text-green-400',
            action: { type: 'quest', action: 'decrypt', args: [key] },
            sound: 'success',
          };
        }
        return {
          output: ['> Неверный ключ. Попробуйте найти его в файлах.'],
          color: 'text-red-400',
          sound: 'error',
        };
      }
      case 'hack': {
        const target = typeof args[0] === 'string' ? args[0] : 'firewall';
        return {
          output: [`> Взлом ${target}...`, '> Требуется мини-игра для завершения!'],
          color: 'text-amber-400',
          action: { type: 'quest', action: 'hack', args: [target] },
        };
      }
      case 'debug':
        return {
          output: ['> Режим отладки активирован.', '> Найдено 5 багов в секторе.'],
          color: 'text-yellow-400',
          action: { type: 'quest', action: 'debug' },
        };
      case 'patch':
        return {
          output: ['> Патч применён. Один баг исправлен.'],
          color: 'text-green-400',
          action: { type: 'quest', action: 'patch' },
          sound: 'success',
        };
      case 'escape':
        return {
          output: ['> Попытка побега...'],
          color: 'text-purple-400',
          action: { type: 'quest', action: 'escape' },
          sound: 'escape',
        };
      case 'inventory':
        return {
          output: ['> Инвентарь пуст. Собирайте предметы в квесте.'],
          color: 'text-gray-400',
          action: { type: 'quest', action: 'inventory' },
        };
      case 'nash':
        return {
          output: [
            '> === Равновесие Нэша ===',
            '  Джон Нэш (1928-2015) доказал, что в любой',
            '  конечной игре существует равновесие — набор',
            '  стратегий, при котором никто не может',
            '  улучшить результат в одностороннем порядке.',
            '  За это он получил Нобелевскую премию в 1994.',
          ],
          color: 'text-cyan-400',
          action: { type: 'invader', pose: 'idle', mood: 'happy' },
        };
      case 'stackoverflow':
        return {
          output: [
            '> [Закрыт] Как сбежать из кода?',
            '  Помечен как дубликат.',
            '  "Этот вопрос уже задавался ранее."',
            '  -1 Покажите что вы пробовали.',
          ],
          color: 'text-amber-400',
          action: { type: 'invader', mood: 'scared' },
        };
      default:
        return {
          output: [`> Неизвестная функция: ${method}()`, '  Введите help для списка команд.'],
          color: 'text-red-400',
          sound: 'error',
        };
    }
  }

  // --- Simple commands ---
  if (object === '_simple') {
    switch (method) {
      case 'help': {
        if (args[0] === 'invader') {
          return {
            output: [
              '> === Команды инвейдера ===',
              '  invader.jump(n)        — прыжок (n раз)',
              '  invader.spin(deg)      — вращение',
              '  invader.say("текст")   — речь',
              '  invader.dance()        — танец',
              '  invader.fly()          — полёт',
              '  invader.attack()       — атака',
              '  invader.shield()       — щит',
              '  invader.sleep()        — сон',
              '  invader.cry()          — плач',
              '  invader.teleport()     — телепорт',
              '  invader.transform("x") — трансформация',
              '  invader.status()       — статус',
            ],
            color: 'text-gray-300',
          };
        }
        return {
          output: [
            '> === Команды ===',
            '  invader.*     — управление инвейдером',
            '  help invader  — подробнее',
            '  ls / cat      — файловая система',
            '  scan()        — сканирование',
            '  play.*        — мини-игры',
            '  quest.start() — начать квест',
            '  clear         — очистить терминал',
            '  whoami        — кто вы',
          ],
          color: 'text-gray-300',
        };
      }
      case 'start':
        return {
          output: ['> Запуск квеста...', '> [Уровень 1: Разведка]'],
          color: 'text-purple-400',
          action: { type: 'quest', action: 'start' },
          sound: 'levelUp',
        };
      case 'clear':
        return {
          output: [],
          action: { type: 'terminal', action: 'clear' },
        };
      case 'history':
        return {
          output: ['> (история отображается отдельно)'],
          action: { type: 'terminal', action: 'history' },
        };
      case 'whoami':
        return {
          output: [
            '> user@tjudge-terminal',
            '  Role: hacker',
            '  Status: в поисках инвейдера',
          ],
          color: 'text-green-400',
        };
      case 'ls': {
        const dir = typeof args[0] === 'string' ? args[0] : '';
        if (dir === 'logs' || dir === 'logs/') {
          return {
            output: ['  error.log  access.log'],
            color: 'text-cyan-400',
          };
        }
        if (dir === 'games' || dir === 'games/') {
          return {
            output: ['  pong.exe  strategy.dat'],
            color: 'text-cyan-400',
          };
        }
        return {
          output: [
            '  readme.txt   config.sys   .hidden',
            '  logs/        games/',
          ],
          color: 'text-cyan-400',
        };
      }
      case 'cat': {
        const file = args.join(' ');
        const content = VIRTUAL_FS[file];
        if (content) {
          return {
            output: content.split('\n').map((l) => `  ${l}`),
            color: 'text-gray-300',
            sound: 'click',
          };
        }
        return {
          output: [`> cat: ${file}: нет такого файла`],
          color: 'text-red-400',
          sound: 'error',
        };
      }
      case 'ping':
        return {
          output: [
            '> PING tjudge.io (10.0.0.42):',
            '  64 bytes: time=13ms',
            '  64 bytes: time=11ms',
            '  64 bytes: time=12ms',
            '  --- 3 packets, 0% loss ---',
          ],
          color: 'text-green-400',
        };
      case 'pwd':
        return {
          output: ['  /home/hacker/tjudge'],
          color: 'text-gray-400',
        };
      case 'date':
        return {
          output: [`  ${new Date().toLocaleString('ru-RU')}`],
          color: 'text-gray-400',
        };
      default:
        return {
          output: [`> Команда не найдена: ${method}`, '  Введите help для справки.'],
          color: 'text-red-400',
          sound: 'error',
        };
    }
  }

  return {
    output: [`> Неизвестная команда: ${input}`, '  Введите help для справки.'],
    color: 'text-red-400',
    sound: 'error',
  };
}

/** Tab-complete: return matching commands for partial input */
export function tabComplete(partial: string): string[] {
  if (!partial) return [];
  const lower = partial.toLowerCase();
  return ALL_COMMANDS.filter((cmd) => cmd.toLowerCase().startsWith(lower));
}
