import { useState, useRef, useEffect, useCallback, type KeyboardEvent } from 'react';
import type { QuestState, QuestAction, TerminalLine } from '../../hooks/useQuestState';
import { tabComplete, executeCommand as execCmd } from '../../utils/commandParser';
import { sound } from '../../utils/soundManager';

// --- Syntax highlight tokens in input ---

function highlightInput(text: string): React.ReactNode[] {
  const parts: React.ReactNode[] = [];
  let remaining = text;
  let key = 0;

  while (remaining.length > 0) {
    // String literals
    const strMatch = remaining.match(/^(["'])(?:(?!\1).)*\1/);
    if (strMatch) {
      parts.push(<span key={key++} className="text-amber-400">{strMatch[0]}</span>);
      remaining = remaining.slice(strMatch[0].length);
      continue;
    }

    // Numbers
    const numMatch = remaining.match(/^\d+/);
    if (numMatch) {
      parts.push(<span key={key++} className="text-cyan-400">{numMatch[0]}</span>);
      remaining = remaining.slice(numMatch[0].length);
      continue;
    }

    // Known objects (before dot)
    const objMatch = remaining.match(/^(invader|play|quest|game|terminal)\b/);
    if (objMatch) {
      parts.push(<span key={key++} className="text-purple-400">{objMatch[0]}</span>);
      remaining = remaining.slice(objMatch[0].length);
      continue;
    }

    // Method after dot
    const dotMethodMatch = remaining.match(/^\.(\w+)/);
    if (dotMethodMatch) {
      parts.push(<span key={key++} className="text-gray-500">.</span>);
      parts.push(<span key={key++} className="text-green-400">{dotMethodMatch[1]}</span>);
      remaining = remaining.slice(dotMethodMatch[0].length);
      continue;
    }

    // Known keywords
    const kwMatch = remaining.match(/^(help|clear|ls|cat|whoami|ping|scan|decrypt|hack|debug|patch|escape|inventory|start|history|import|sudo|git|nash|stackoverflow)\b/);
    if (kwMatch) {
      parts.push(<span key={key++} className="text-green-400">{kwMatch[0]}</span>);
      remaining = remaining.slice(kwMatch[0].length);
      continue;
    }

    // Parens
    if (remaining[0] === '(' || remaining[0] === ')') {
      parts.push(<span key={key++} className="text-gray-500">{remaining[0]}</span>);
      remaining = remaining.slice(1);
      continue;
    }

    // Default character
    parts.push(<span key={key++} className="text-gray-300">{remaining[0]}</span>);
    remaining = remaining.slice(1);
  }

  return parts;
}

// --- Typing animation for a single line ---

function TypingLine({ line, onDone }: { line: TerminalLine; onDone?: () => void }) {
  const [displayed, setDisplayed] = useState('');
  const [done, setDone] = useState(!line.typing);

  useEffect(() => {
    if (!line.typing) {
      setDisplayed(line.text);
      setDone(true);
      return;
    }
    let i = 0;
    setDisplayed('');
    setDone(false);
    const interval = setInterval(() => {
      i++;
      setDisplayed(line.text.slice(0, i));
      if (i >= line.text.length) {
        clearInterval(interval);
        setDone(true);
        onDone?.();
      }
    }, 25);
    return () => clearInterval(interval);
  }, [line.text, line.typing, onDone]);

  return (
    <div className={`${line.color} whitespace-pre leading-relaxed`}>
      {line.isCommand ? (
        <span className="text-gray-600">$ </span>
      ) : null}
      {displayed}
      {!done && <span className="terminal-cursor">_</span>}
    </div>
  );
}

// --- Tab completion popup ---

function TabPopup({ matches, selected }: { matches: string[]; selected: number }) {
  if (matches.length === 0) return null;
  const maxVisible = 8;
  const start = Math.max(0, Math.min(selected - 3, matches.length - maxVisible));
  const end = Math.min(matches.length, start + maxVisible);
  const visible = matches.slice(start, end);

  return (
    <div className="mb-1 bg-gray-800 border border-gray-700 rounded-lg p-2 text-xs max-w-sm">
      {start > 0 && <div className="text-gray-600 px-2 mb-0.5">...</div>}
      {visible.map((m, i) => (
        <div
          key={m}
          className={`px-2 py-0.5 rounded ${start + i === selected ? 'bg-primary-600 text-white' : 'text-gray-400'}`}
        >
          {m}
        </div>
      ))}
      {end < matches.length && (
        <div className="text-gray-600 px-2 mt-0.5">...ещё {matches.length - end}</div>
      )}
    </div>
  );
}

// --- Main terminal component ---

interface QuestTerminalProps {
  state: QuestState;
  dispatch: React.Dispatch<QuestAction>;
}

export function QuestTerminal({ state, dispatch }: QuestTerminalProps) {
  const [input, setInput] = useState('');
  const [tabMatches, setTabMatches] = useState<string[]>([]);
  const [tabIndex, setTabIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom (also when tab popup appears)
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [state.terminalLines, tabMatches]);

  // Focus input on click anywhere in terminal
  const focusInput = useCallback(() => {
    inputRef.current?.focus({ preventScroll: true });
  }, []);

  // Handle history navigation from state
  useEffect(() => {
    if (state.historyIndex >= 0 && state.historyIndex < state.commandHistory.length) {
      setInput(state.commandHistory[state.historyIndex]);
    } else if (state.historyIndex === -1 && state.commandHistory.length > 0) {
      // Only clear if we navigated down past the last entry
    }
  }, [state.historyIndex, state.commandHistory]);

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    // Tab completion
    if (e.key === 'Tab') {
      e.preventDefault();
      if (tabMatches.length === 0) {
        const matches = tabComplete(input);
        if (matches.length === 1) {
          setInput(matches[0]);
          setTabMatches([]);
        } else if (matches.length > 1) {
          setTabMatches(matches);
          setTabIndex(0);
          setInput(matches[0]);
        }
      } else {
        // Cycle through matches
        const nextIdx = (tabIndex + 1) % tabMatches.length;
        setTabIndex(nextIdx);
        setInput(tabMatches[nextIdx]);
      }
      sound.click();
      return;
    }

    // Clear tab matches on any other key
    if (tabMatches.length > 0 && e.key !== 'Tab') {
      setTabMatches([]);
    }

    // Enter = execute
    if (e.key === 'Enter') {
      e.preventDefault();
      if (input.trim()) {
        dispatch({ type: 'EXECUTE_COMMAND', command: input });

        // Play sound based on result (check command parser directly)
        const result = execCmd(input);
        if (result.sound === 'success') sound.success();
        else if (result.sound === 'error') sound.error();
        else if (result.sound === 'levelUp') sound.levelUp();
        else if (result.sound === 'escape') sound.escape();
        else sound.click();

        setInput('');
      }
      return;
    }

    // Arrow up/down = history
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      dispatch({ type: 'NAVIGATE_HISTORY', direction: 'up' });
      return;
    }
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      dispatch({ type: 'NAVIGATE_HISTORY', direction: 'down' });
      return;
    }
  };

  // Limit displayed lines for performance
  const visibleLines = state.terminalLines.slice(-80);

  return (
    <div
      className="bg-gray-900/80 border border-gray-800 rounded-xl overflow-hidden backdrop-blur-sm"
      onClick={focusInput}
    >
      {/* macOS-style chrome */}
      <div className="flex items-center gap-2 px-4 py-2.5 bg-gray-900 border-b border-gray-800">
        <div className="flex gap-1.5">
          <div className="w-3 h-3 rounded-full bg-red-500/80" />
          <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
          <div className="w-3 h-3 rounded-full bg-green-500/80" />
        </div>
        <span className="ml-3 text-xs text-gray-500 font-mono">
          tjudge-quest
          {state.level > 0 && ` — Уровень ${state.level}`}
        </span>
        {state.level > 0 && (
          <span className="ml-auto text-xs text-gray-600 font-mono">
            {state.objectives.filter((o) => o.completed).length}/{state.objectives.length}
          </span>
        )}
      </div>

      {/* Terminal body */}
      <div
        ref={scrollRef}
        className="p-4 font-mono text-sm overflow-y-auto"
        style={{ height: '340px', lineHeight: '1.6' }}
      >
        {visibleLines.map((line, i) => (
          <TypingLine key={`${i}-${line.text.slice(0, 20)}`} line={line} />
        ))}

        {/* Objective tracker (compact) */}
        {state.level > 0 && state.objectives.length > 0 && (
          <div className="mt-2 mb-1 border-t border-gray-800 pt-2">
            {state.objectives.map((obj) => (
              <div key={obj.id} className={`text-xs ${obj.completed ? 'text-green-500' : 'text-gray-600'}`}>
                {obj.completed ? '[x]' : '[ ]'} {obj.text}
              </div>
            ))}
          </div>
        )}

        {/* Tab completion popup — in flow above input to avoid clipping by overflow container */}
        <TabPopup matches={tabMatches} selected={tabIndex} />

        {/* Input line */}
        <div className="flex items-center gap-1 mt-1">
          <span className="text-green-500 select-none">$</span>
          <div className="flex-1 relative">
            {/* Syntax-highlighted overlay */}
            <div className="absolute inset-0 pointer-events-none whitespace-pre font-mono text-sm leading-6 pl-1">
              {highlightInput(input)}
            </div>
            {/* Actual input (transparent text) */}
            <input
              ref={inputRef}
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              className="w-full bg-transparent text-transparent caret-green-400 outline-none focus-visible:ring-1 focus-visible:ring-green-500/50 font-mono text-sm leading-6 pl-1"
              spellCheck={false}
              autoComplete="off"
            />
          </div>
          {!input && <span className="terminal-cursor text-green-400">_</span>}
        </div>
      </div>
    </div>
  );
}
