import { useReducer, useCallback } from 'react';
import { executeCommand, type CommandResult, type CommandAction } from '../utils/commandParser';
import type { InvaderPose } from '../components/SpaceInvader';

// --- Types ---

export interface TerminalLine {
  text: string;
  color: string;
  isCommand?: boolean;
  typing?: boolean;
}

export interface Objective {
  id: string;
  text: string;
  completed: boolean;
}

export interface QuestState {
  level: number; // 0 = free play, 1-5 = story
  phase: 'free' | 'story' | 'minigame' | 'cutscene';
  objectives: Objective[];
  completedObjectives: string[];
  inventory: string[];
  invaderMood: 'happy' | 'neutral' | 'scared' | 'angry' | 'sleepy';
  invaderPose: InvaderPose;
  invaderSpeech: string | null;
  invaderTransform: 'fire' | 'ice' | 'ghost' | 'rainbow' | null;
  invaderJump: number;
  escaping: boolean;
  terminalLines: TerminalLine[];
  commandHistory: string[];
  historyIndex: number;
  activeGame: 'pong' | 'typing' | 'strategy' | null;
}

export type QuestAction =
  | { type: 'EXECUTE_COMMAND'; command: string }
  | { type: 'COMPLETE_OBJECTIVE'; id: string }
  | { type: 'ADVANCE_LEVEL' }
  | { type: 'ADD_OUTPUT'; lines: TerminalLine[] }
  | { type: 'START_MINIGAME'; game: 'pong' | 'typing' | 'strategy' }
  | { type: 'END_MINIGAME'; result: 'win' | 'lose' }
  | { type: 'CLEAR_TERMINAL' }
  | { type: 'NAVIGATE_HISTORY'; direction: 'up' | 'down' }
  | { type: 'SET_POSE'; pose: InvaderPose }
  | { type: 'RESET_INVADER' }
  | { type: 'CLEAR_SPEECH' };

// --- Level definitions ---

const LEVEL_OBJECTIVES: Record<number, Objective[]> = {
  1: [
    { id: 'scan', text: 'Просканируйте систему: scan()', completed: false },
    { id: 'read_hidden', text: 'Найдите секретный файл: cat .hidden', completed: false },
    { id: 'read_logs', text: 'Прочитайте логи: cat logs/error.log', completed: false },
  ],
  2: [
    { id: 'decrypt', text: 'Расшифруйте ключ: decrypt("NASH-1950")', completed: false },
    { id: 'attack', text: 'Атакуйте файрвол: invader.attack()', completed: false },
    { id: 'pong', text: 'Победите в Pong: play.pong()', completed: false },
  ],
  3: [
    { id: 'debug', text: 'Найдите баги: debug()', completed: false },
    { id: 'shield', text: 'Активируйте щит: invader.shield()', completed: false },
    { id: 'typing', text: 'Исправьте код: play.typing()', completed: false },
  ],
  4: [
    { id: 'sleep', text: 'Усыпите охрану: invader.sleep()', completed: false },
    { id: 'teleport', text: 'Телепортируйтесь: invader.teleport()', completed: false },
    { id: 'strategy', text: 'Обманите стражу: play.strategy()', completed: false },
  ],
  5: [
    { id: 'fly', text: 'Активируйте полёт: invader.fly()', completed: false },
    { id: 'transform', text: 'Трансформируйтесь: invader.transform("радуга")', completed: false },
    { id: 'escape', text: 'Сбегите: escape()', completed: false },
  ],
};

const LEVEL_INTROS: Record<number, TerminalLine[]> = {
  1: [
    { text: '='.repeat(40), color: 'text-purple-500' },
    { text: '  УРОВЕНЬ 1: РАЗВЕДКА', color: 'text-purple-400', typing: true },
    { text: '='.repeat(40), color: 'text-purple-500' },
    { text: '', color: 'text-gray-500' },
    { text: '  Инвейдер сбежал из исходного кода.', color: 'text-gray-300', typing: true },
    { text: '  Исследуйте систему, найдите его следы.', color: 'text-gray-300', typing: true },
    { text: '', color: 'text-gray-500' },
    { text: '  Цели:', color: 'text-yellow-400' },
    { text: '  [ ] scan() — просканировать систему', color: 'text-gray-400' },
    { text: '  [ ] cat .hidden — найти секрет', color: 'text-gray-400' },
    { text: '  [ ] cat logs/error.log — прочитать логи', color: 'text-gray-400' },
  ],
  2: [
    { text: '='.repeat(40), color: 'text-red-500' },
    { text: '  УРОВЕНЬ 2: ФАЙРВОЛ', color: 'text-red-400', typing: true },
    { text: '='.repeat(40), color: 'text-red-500' },
    { text: '', color: 'text-gray-500' },
    { text: '  Путь преграждает файрвол.', color: 'text-gray-300', typing: true },
    { text: '  Расшифруйте ключ и прорвитесь.', color: 'text-gray-300', typing: true },
  ],
  3: [
    { text: '='.repeat(40), color: 'text-yellow-500' },
    { text: '  УРОВЕНЬ 3: ЗОНА БАГОВ', color: 'text-yellow-400', typing: true },
    { text: '='.repeat(40), color: 'text-yellow-500' },
    { text: '', color: 'text-gray-500' },
    { text: '  Код повреждён. Исправьте баги.', color: 'text-gray-300', typing: true },
  ],
  4: [
    { text: '='.repeat(40), color: 'text-blue-500' },
    { text: '  УРОВЕНЬ 4: ОХРАНА', color: 'text-blue-400', typing: true },
    { text: '='.repeat(40), color: 'text-blue-500' },
    { text: '', color: 'text-gray-500' },
    { text: '  Охранные системы на страже.', color: 'text-gray-300', typing: true },
    { text: '  Проберитесь незаметно.', color: 'text-gray-300', typing: true },
  ],
  5: [
    { text: '='.repeat(40), color: 'text-green-500' },
    { text: '  УРОВЕНЬ 5: ПОБЕГ', color: 'text-green-400', typing: true },
    { text: '='.repeat(40), color: 'text-green-500' },
    { text: '', color: 'text-gray-500' },
    { text: '  Финальный уровень! Сбегите из кода.', color: 'text-gray-300', typing: true },
    { text: '  Инвейдер вырвется за пределы секции!', color: 'text-gray-300', typing: true },
  ],
};

// --- Initial state ---

const WELCOME_LINES: TerminalLine[] = [
  { text: '> TJudge Terminal v3.7', color: 'text-purple-400' },
  { text: '> Введите help для списка команд', color: 'text-gray-500' },
  { text: '> Или quest.start() для начала квеста', color: 'text-gray-500' },
  { text: '', color: 'text-gray-500' },
];

const initialState: QuestState = {
  level: 0,
  phase: 'free',
  objectives: [],
  completedObjectives: [],
  inventory: [],
  invaderMood: 'neutral',
  invaderPose: 'idle',
  invaderSpeech: null,
  invaderTransform: null,
  invaderJump: 0,
  escaping: false,
  terminalLines: [...WELCOME_LINES],
  commandHistory: [],
  historyIndex: -1,
  activeGame: null,
};

// --- Check if command completes an objective ---

function checkObjective(state: QuestState, command: string, result: CommandResult): string | null {
  if (state.level === 0) return null;

  const lower = command.trim().toLowerCase();
  const objectives = state.objectives;

  for (const obj of objectives) {
    if (obj.completed) continue;

    switch (obj.id) {
      case 'scan':
        if (result.action?.type === 'quest' && (result.action as { action: string }).action === 'scan') return 'scan';
        break;
      case 'read_hidden':
        if (lower.includes('cat') && lower.includes('.hidden')) return 'read_hidden';
        break;
      case 'read_logs':
        if (lower.includes('cat') && lower.includes('error.log')) return 'read_logs';
        break;
      case 'decrypt':
        if (result.action?.type === 'quest' && (result.action as { action: string }).action === 'decrypt' && result.sound === 'success') return 'decrypt';
        break;
      case 'attack':
        if (result.action?.type === 'invader' && (result.action as { pose?: string }).pose === 'attack') return 'attack';
        break;
      case 'pong':
      case 'typing':
      case 'strategy':
        // These are completed via END_MINIGAME
        break;
      case 'debug':
        if (result.action?.type === 'quest' && (result.action as { action: string }).action === 'debug') return 'debug';
        break;
      case 'shield':
        if (result.action?.type === 'invader' && (result.action as { pose?: string }).pose === 'shield') return 'shield';
        break;
      case 'sleep':
        if (result.action?.type === 'invader' && (result.action as { pose?: string }).pose === 'sleep') return 'sleep';
        break;
      case 'teleport':
        if (result.action?.type === 'invader' && (result.action as { pose?: string }).pose === 'teleport') return 'teleport';
        break;
      case 'fly':
        if (result.action?.type === 'invader' && (result.action as { pose?: string }).pose === 'fly') return 'fly';
        break;
      case 'transform':
        if (result.action?.type === 'invader' && (result.action as { pose?: string }).pose === 'transform') return 'transform';
        break;
      case 'escape':
        if (result.action?.type === 'quest' && (result.action as { action: string }).action === 'escape') return 'escape';
        break;
    }
  }
  return null;
}

// --- Apply invader action to state ---

function applyInvaderAction(state: QuestState, action: CommandAction): Partial<QuestState> {
  if (action.type !== 'invader') return {};
  const ia = action;
  const updates: Partial<QuestState> = {};

  if (ia.pose) updates.invaderPose = ia.pose;
  if (ia.mood) updates.invaderMood = ia.mood;
  if (ia.say) updates.invaderSpeech = ia.say;
  if (ia.transform !== undefined) updates.invaderTransform = ia.transform;
  if (ia.jump) updates.invaderJump = (state.invaderJump || 0) + 1; // trigger change

  return updates;
}

// --- Reducer ---

function questReducer(state: QuestState, action: QuestAction): QuestState {
  switch (action.type) {
    case 'EXECUTE_COMMAND': {
      const cmd = action.command.trim();
      if (!cmd) return state;

      const result = executeCommand(cmd);

      // Build command echo line
      const newLines: TerminalLine[] = [
        { text: `$ ${cmd}`, color: 'text-gray-500', isCommand: true },
      ];

      // Add output lines
      result.output.forEach((text) => {
        newLines.push({ text, color: result.color || 'text-gray-300' });
      });

      // Check for terminal clear
      if (result.action?.type === 'terminal' && result.action.action === 'clear') {
        return {
          ...state,
          terminalLines: [],
          commandHistory: [...state.commandHistory, cmd],
          historyIndex: -1,
        };
      }

      // Check for quest start
      if (result.action?.type === 'quest' && result.action.action === 'start' && state.level === 0) {
        const introLines = LEVEL_INTROS[1] || [];
        return {
          ...state,
          level: 1,
          phase: 'story',
          objectives: LEVEL_OBJECTIVES[1].map((o) => ({ ...o })),
          terminalLines: [...state.terminalLines, ...newLines, ...introLines],
          commandHistory: [...state.commandHistory, cmd],
          historyIndex: -1,
        };
      }

      // Check for history display
      if (result.action?.type === 'terminal' && result.action.action === 'history') {
        const histLines = state.commandHistory.map((c, i) => ({
          text: `  ${i + 1}  ${c}`,
          color: 'text-gray-400',
        }));
        return {
          ...state,
          terminalLines: [
            ...state.terminalLines,
            { text: '$ history', color: 'text-gray-500', isCommand: true },
            ...histLines,
          ],
          commandHistory: [...state.commandHistory, cmd],
          historyIndex: -1,
        };
      }

      // Check for minigame launch
      if (result.action?.type === 'game') {
        return {
          ...state,
          phase: 'minigame',
          activeGame: result.action.game,
          terminalLines: [...state.terminalLines, ...newLines],
          commandHistory: [...state.commandHistory, cmd],
          historyIndex: -1,
        };
      }

      // Apply invader actions
      let invaderUpdates: Partial<QuestState> = {};
      if (result.action) {
        invaderUpdates = applyInvaderAction(state, result.action);
      }

      // Check for escape action at level 5
      let escaping = state.escaping;
      if (state.level === 5 && result.action?.type === 'quest' && result.action.action === 'escape') {
        escaping = true;
      }

      // Check objective completion
      let objectives = state.objectives;
      const completedObjectives = [...state.completedObjectives];
      const completedId = checkObjective(state, cmd, result);
      if (completedId && !completedObjectives.includes(completedId)) {
        completedObjectives.push(completedId);
        objectives = objectives.map((o) =>
          o.id === completedId ? { ...o, completed: true } : o
        );
        newLines.push({ text: `  [x] Цель выполнена!`, color: 'text-green-400' });
      }

      // Check if all objectives completed → advance level
      const allDone = objectives.length > 0 && objectives.every((o) => o.completed);
      let nextLevel = state.level;
      if (allDone && state.level > 0 && state.level < 5) {
        nextLevel = state.level + 1;
        const introLines = LEVEL_INTROS[nextLevel] || [];
        newLines.push(
          { text: '', color: 'text-gray-500' },
          { text: `> Уровень ${state.level} пройден!`, color: 'text-green-400' },
          ...introLines
        );
        return {
          ...state,
          ...invaderUpdates,
          level: nextLevel,
          objectives: (LEVEL_OBJECTIVES[nextLevel] || []).map((o) => ({ ...o })),
          completedObjectives,
          escaping,
          terminalLines: [...state.terminalLines, ...newLines],
          commandHistory: [...state.commandHistory, cmd],
          historyIndex: -1,
        };
      }

      // Level 5 complete
      if (allDone && state.level === 5) {
        newLines.push(
          { text: '', color: 'text-gray-500' },
          { text: '='.repeat(40), color: 'text-green-500' },
          { text: '  ИНВЕЙДЕР СБЕЖАЛ!', color: 'text-green-400', typing: true },
          { text: '  Квест завершён. Поздравляем!', color: 'text-green-400', typing: true },
          { text: '='.repeat(40), color: 'text-green-500' },
        );
      }

      return {
        ...state,
        ...invaderUpdates,
        objectives,
        completedObjectives,
        escaping,
        terminalLines: [...state.terminalLines, ...newLines],
        commandHistory: [...state.commandHistory, cmd],
        historyIndex: -1,
      };
    }

    case 'COMPLETE_OBJECTIVE': {
      const objectives = state.objectives.map((o) =>
        o.id === action.id ? { ...o, completed: true } : o
      );
      return {
        ...state,
        objectives,
        completedObjectives: [...state.completedObjectives, action.id],
      };
    }

    case 'ADVANCE_LEVEL': {
      const next = state.level + 1;
      if (next > 5) return state;
      return {
        ...state,
        level: next,
        objectives: (LEVEL_OBJECTIVES[next] || []).map((o) => ({ ...o })),
      };
    }

    case 'ADD_OUTPUT':
      return {
        ...state,
        terminalLines: [...state.terminalLines, ...action.lines],
      };

    case 'START_MINIGAME':
      return { ...state, phase: 'minigame', activeGame: action.game };

    case 'END_MINIGAME': {
      const gameId = state.activeGame;
      let objectives = state.objectives;
      const completedObjectives = [...state.completedObjectives];
      const lines: TerminalLine[] = [];

      if (action.result === 'win' && gameId && !completedObjectives.includes(gameId)) {
        completedObjectives.push(gameId);
        objectives = objectives.map((o) =>
          o.id === gameId ? { ...o, completed: true } : o
        );
        lines.push(
          { text: `> Мини-игра пройдена!`, color: 'text-green-400' },
          { text: `  [x] Цель выполнена!`, color: 'text-green-400' },
        );
      } else if (action.result === 'lose') {
        lines.push({ text: `> Попробуйте ещё раз.`, color: 'text-yellow-400' });
      }

      // Check if all objectives done after minigame win
      const allDone = objectives.length > 0 && objectives.every((o) => o.completed);
      let nextLevel = state.level;
      if (allDone && state.level > 0 && state.level < 5) {
        nextLevel = state.level + 1;
        const introLines = LEVEL_INTROS[nextLevel] || [];
        lines.push(
          { text: '', color: 'text-gray-500' },
          { text: `> Уровень ${state.level} пройден!`, color: 'text-green-400' },
          ...introLines
        );
        return {
          ...state,
          phase: 'story',
          activeGame: null,
          level: nextLevel,
          objectives: (LEVEL_OBJECTIVES[nextLevel] || []).map((o) => ({ ...o })),
          completedObjectives,
          terminalLines: [...state.terminalLines, ...lines],
        };
      }

      if (allDone && state.level === 5) {
        lines.push(
          { text: '', color: 'text-gray-500' },
          { text: '  ИНВЕЙДЕР СБЕЖАЛ! Квест завершён!', color: 'text-green-400', typing: true },
        );
      }

      return {
        ...state,
        phase: state.level > 0 ? 'story' : 'free',
        activeGame: null,
        objectives,
        completedObjectives,
        terminalLines: [...state.terminalLines, ...lines],
      };
    }

    case 'CLEAR_TERMINAL':
      return { ...state, terminalLines: [] };

    case 'NAVIGATE_HISTORY': {
      const history = state.commandHistory;
      if (history.length === 0) return state;
      let idx = state.historyIndex;
      if (action.direction === 'up') {
        idx = idx === -1 ? history.length - 1 : Math.max(0, idx - 1);
      } else {
        idx = idx === -1 ? -1 : idx >= history.length - 1 ? -1 : idx + 1;
      }
      return { ...state, historyIndex: idx };
    }

    case 'SET_POSE':
      return { ...state, invaderPose: action.pose };

    case 'RESET_INVADER':
      return {
        ...state,
        invaderPose: 'idle',
        invaderMood: 'neutral',
        invaderSpeech: null,
        invaderTransform: null,
      };

    case 'CLEAR_SPEECH':
      return { ...state, invaderSpeech: null };

    default:
      return state;
  }
}

// --- Hook ---

export function useQuestState() {
  const [state, dispatch] = useReducer(questReducer, initialState);

  const execute = useCallback((command: string) => {
    dispatch({ type: 'EXECUTE_COMMAND', command });
  }, []);

  return { state, dispatch, execute } as const;
}
