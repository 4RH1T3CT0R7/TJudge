import { useState, useEffect, useCallback, useRef } from 'react';
import { sound } from '../../utils/soundManager';

// --- ASCII Pong ---

interface PongProps {
  onEnd: (result: 'win' | 'lose') => void;
}

function AsciiPong({ onEnd }: PongProps) {
  const WIDTH = 24;
  const HEIGHT = 10;
  const [playerY, setPlayerY] = useState(4);
  const [aiY, setAiY] = useState(4);
  const [ballX, setBallX] = useState(12);
  const [ballY, setBallY] = useState(5);
  const [ballDx, setBallDx] = useState(1);
  const [ballDy, setBallDy] = useState(1);
  const [playerScore, setPlayerScore] = useState(0);
  const [aiScore, setAiScore] = useState(0);
  const [running, setRunning] = useState(true);
  const frameRef = useRef<ReturnType<typeof setInterval>>(undefined);

  // Game loop
  useEffect(() => {
    if (!running) return;
    frameRef.current = setInterval(() => {
      setBallX((x) => x + ballDx);
      setBallY((y) => {
        let ny = y + ballDy;
        if (ny <= 0 || ny >= HEIGHT - 1) {
          setBallDy((d) => -d);
          ny = Math.max(0, Math.min(HEIGHT - 1, ny));
        }
        return ny;
      });
    }, 200);
    return () => clearInterval(frameRef.current);
  }, [running, ballDx, ballDy]);

  // Collision detection — game loop requires synchronous state updates
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    // Player paddle (left side, x=1)
    if (ballX <= 1) {
      if (Math.abs(ballY - playerY) <= 1) {
        setBallDx(1);
        sound.click();
      } else {
        setAiScore((s) => s + 1);
        setBallX(12);
        setBallY(5);
        setBallDx(1);
        sound.error();
      }
    }
    // AI paddle (right side, x=WIDTH-2)
    if (ballX >= WIDTH - 2) {
      if (Math.abs(ballY - aiY) <= 1) {
        setBallDx(-1);
        sound.click();
      } else {
        setPlayerScore((s) => s + 1);
        setBallX(12);
        setBallY(5);
        setBallDx(-1);
        sound.success();
      }
    }
  }, [ballX, ballY, playerY, aiY]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // AI movement
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => {
      setAiY((y) => {
        if (ballY > y) return Math.min(HEIGHT - 2, y + 1);
        if (ballY < y) return Math.max(1, y - 1);
        return y;
      });
    }, 300);
    return () => clearInterval(t);
  }, [running, ballY]);

  // Check win/lose — game loop state transitions
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    if (playerScore >= 3) {
      setRunning(false);
      sound.levelUp();
      onEnd('win');
    } else if (aiScore >= 3) {
      setRunning(false);
      sound.error();
      onEnd('lose');
    }
  }, [playerScore, aiScore, onEnd]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // Keyboard controls
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'w' || e.key === 'ArrowUp') {
        e.preventDefault();
        setPlayerY((y) => Math.max(1, y - 1));
      }
      if (e.key === 's' || e.key === 'ArrowDown') {
        e.preventDefault();
        setPlayerY((y) => Math.min(HEIGHT - 2, y + 1));
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  // Render grid
  const grid = Array.from({ length: HEIGHT }, (_, y) =>
    Array.from({ length: WIDTH }, (_, x) => {
      if (x === 0 || x === WIDTH - 1) return '|';
      if (y === 0 || y === HEIGHT - 1) return '-';
      if (x === 1 && Math.abs(y - playerY) <= 1) return '|';
      if (x === WIDTH - 2 && Math.abs(y - aiY) <= 1) return '|';
      if (x === Math.round(ballX) && y === Math.round(ballY)) return 'o';
      return ' ';
    }).join('')
  );

  return (
    <div className="space-y-1">
      <div className="text-xs text-gray-500 mb-2">
        [w/s или arrows] Вы: {playerScore} | Файрвол: {aiScore} | До 3
      </div>
      <pre className="text-green-400 text-xs leading-tight">
        {grid.join('\n')}
      </pre>
      {!running && (
        <div className={`text-sm font-bold mt-2 ${playerScore >= 3 ? 'text-green-400' : 'text-red-400'}`}>
          {playerScore >= 3 ? 'Победа! Файрвол взломан.' : 'Поражение. Попробуйте ещё.'}
        </div>
      )}
    </div>
  );
}

// --- Typing Race ---

interface TypingRaceProps {
  onEnd: (result: 'win' | 'lose') => void;
}

const CODE_LINES = [
  'if (score > max) return true;',
  'const result = await fetch(url);',
  'for (let i = 0; i < n; i++) {}',
  'function solve(a, b) { return a; }',
  'while (queue.length > 0) pop();',
  'const map = new Map<string, int>();',
  'try { parse(input); } catch (e) {}',
];

function TypingRace({ onEnd }: TypingRaceProps) {
  const [lines] = useState(() => {
    const shuffled = [...CODE_LINES].sort(() => Math.random() - 0.5);
    return shuffled.slice(0, 5);
  });
  const [currentLine, setCurrentLine] = useState(0);
  const [typed, setTyped] = useState('');
  const [timeLeft, setTimeLeft] = useState(30);
  const [completed, setCompleted] = useState(0);
  const [done, setDone] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  // Timer
  useEffect(() => {
    if (done) return;
    const t = setInterval(() => {
      setTimeLeft((tl) => {
        if (tl <= 1) {
          setDone(true);
          return 0;
        }
        return tl - 1;
      });
    }, 1000);
    return () => clearInterval(t);
  }, [done]);

  // Check win when done
  useEffect(() => {
    if (done) {
      if (completed >= 3) {
        sound.levelUp();
        onEnd('win');
      } else {
        sound.error();
        onEnd('lose');
      }
    }
  }, [done, completed, onEnd]);

  // Focus
  useEffect(() => {
    inputRef.current?.focus();
  }, [currentLine]);

  const handleInput = (val: string) => {
    setTyped(val);
    if (val === lines[currentLine]) {
      sound.success();
      setCompleted((c) => c + 1);
      if (currentLine < lines.length - 1) {
        setCurrentLine((l) => l + 1);
        setTyped('');
      } else {
        setDone(true);
      }
    }
  };

  // Render with diff highlighting
  const target = lines[currentLine] || '';
  const renderTarget = () => {
    return target.split('').map((ch, i) => {
      let cls = 'text-gray-500';
      if (i < typed.length) {
        cls = typed[i] === ch ? 'text-green-400' : 'text-red-400 underline';
      }
      return <span key={i} className={cls}>{ch}</span>;
    });
  };

  return (
    <div className="space-y-2">
      <div className="flex justify-between text-xs text-gray-500">
        <span>Строка {currentLine + 1}/{lines.length}</span>
        <span className={timeLeft <= 5 ? 'text-red-400' : ''}>{timeLeft}s</span>
        <span>Готово: {completed}/5</span>
      </div>
      <div className="bg-gray-800 rounded p-2 font-mono text-sm">
        {renderTarget()}
      </div>
      {!done && (
        <input
          ref={inputRef}
          type="text"
          value={typed}
          onChange={(e) => handleInput(e.target.value)}
          className="w-full bg-transparent border border-gray-700 rounded px-2 py-1 text-green-400 font-mono text-sm outline-none focus-visible:ring-1 focus-visible:ring-green-500/50 focus:border-green-500"
          spellCheck={false}
          autoComplete="off"
        />
      )}
      {done && (
        <div className={`text-sm font-bold ${completed >= 3 ? 'text-green-400' : 'text-red-400'}`}>
          {completed >= 3 ? `Код исправлен! (${completed}/5)` : `Не успели. (${completed}/5)`}
        </div>
      )}
    </div>
  );
}

// --- Guess Strategy ---

interface GuessStrategyProps {
  onEnd: (result: 'win' | 'lose') => void;
}

function GuessStrategy({ onEnd }: GuessStrategyProps) {
  const [round, setRound] = useState(0);
  const [playerMoves, setPlayerMoves] = useState<('C' | 'D')[]>([]);
  const [guardMoves, setGuardMoves] = useState<('C' | 'D')[]>([]);
  const [correct, setCorrect] = useState(0);
  const [done, setDone] = useState(false);
  const [, setLastGuess] = useState<'C' | 'D' | null>(null);

  // Guard uses tit-for-tat with initial cooperate
  const getGuardMove = useCallback((rd: number): 'C' | 'D' => {
    if (rd === 0) return 'C';
    // Tit-for-tat: copy player's last move
    return playerMoves[rd - 1] || 'C';
  }, [playerMoves]);

  const makeGuess = (guess: 'C' | 'D') => {
    if (done) return;
    const guardMove = getGuardMove(round);
    const isCorrect = guess === guardMove;

    setPlayerMoves([...playerMoves, guess]);
    setGuardMoves([...guardMoves, guardMove]);
    setLastGuess(guess);

    if (isCorrect) {
      setCorrect((c) => c + 1);
      sound.success();
    } else {
      sound.error();
    }

    if (round >= 4) {
      setDone(true);
      const finalCorrect = isCorrect ? correct + 1 : correct;
      if (finalCorrect >= 3) {
        sound.levelUp();
        onEnd('win');
      } else {
        onEnd('lose');
      }
    } else {
      setRound((r) => r + 1);
    }
  };

  return (
    <div className="space-y-2">
      <div className="text-xs text-gray-500">
        Угадайте ход стражи: Сотрудничество (C) или Предательство (D)
      </div>
      <div className="text-xs text-gray-600">
        Раунд {round + 1}/5 | Угадано: {correct}
      </div>

      {/* History */}
      {guardMoves.length > 0 && (
        <div className="space-y-0.5">
          {guardMoves.map((gm, i) => (
            <div key={i} className="text-xs font-mono">
              <span className="text-gray-500">R{i + 1}:</span>{' '}
              <span className="text-blue-400">Вы: {playerMoves[i]}</span>{' '}
              <span className="text-red-400">Страж: {gm}</span>{' '}
              <span className={playerMoves[i] === gm ? 'text-green-400' : 'text-red-400'}>
                {playerMoves[i] === gm ? 'OK' : 'X'}
              </span>
            </div>
          ))}
        </div>
      )}

      {!done && (
        <div className="flex gap-2 mt-2">
          <button
            onClick={() => makeGuess('C')}
            className="px-3 py-1 bg-green-900/50 text-green-400 rounded text-sm hover:bg-green-800/50 transition-colors"
          >
            C (Cooperate)
          </button>
          <button
            onClick={() => makeGuess('D')}
            className="px-3 py-1 bg-red-900/50 text-red-400 rounded text-sm hover:bg-red-800/50 transition-colors"
          >
            D (Defect)
          </button>
        </div>
      )}

      {done && (
        <div className={`text-sm font-bold ${correct >= 3 ? 'text-green-400' : 'text-red-400'}`}>
          {correct >= 3 ? `Стража обманута! (${correct}/5)` : `Не удалось. (${correct}/5)`}
        </div>
      )}
    </div>
  );
}

// --- Mini-game wrapper ---

interface MiniGameEngineProps {
  game: 'pong' | 'typing' | 'strategy';
  onEnd: (result: 'win' | 'lose') => void;
}

export function MiniGameEngine({ game, onEnd }: MiniGameEngineProps) {
  const titles: Record<string, string> = {
    pong: 'ASCII Pong vs Firewall',
    typing: 'Typing Race: Fix the Code',
    strategy: 'Guess Strategy: Дилемма стражи',
  };

  return (
    <div className="border border-gray-700 rounded-lg p-3 bg-gray-800/50">
      <div className="text-xs text-purple-400 font-bold mb-2 uppercase tracking-wide">
        {titles[game]}
      </div>
      {game === 'pong' && <AsciiPong onEnd={onEnd} />}
      {game === 'typing' && <TypingRace onEnd={onEnd} />}
      {game === 'strategy' && <GuessStrategy onEnd={onEnd} />}
    </div>
  );
}
