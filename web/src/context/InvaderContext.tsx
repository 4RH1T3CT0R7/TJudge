import { createContext, useContext, useState, useCallback, useRef, type ReactNode } from 'react';
import type { InvaderPose } from '../components/SpaceInvader';

export type InvaderEvent =
  | { type: 'match_win' }
  | { type: 'match_loss' }
  | { type: 'match_draw' }
  | { type: 'rank_up'; delta: number }
  | { type: 'rank_down'; delta: number }
  | { type: 'program_uploaded' }
  | { type: 'program_error'; message: string }
  | { type: 'tournament_started' }
  | { type: 'tournament_completed' }
  | { type: 'team_created'; teamName: string }
  | { type: 'invite_copied' }
  | { type: 'error_400' | 'error_403' | 'error_404' | 'error_500' };

export interface InvaderState {
  pose: InvaderPose;
  speech: string | null;
  eye: 'normal' | 'closed' | 'sad' | 'wide' | null;
  shake: boolean;
  jump: boolean;
}

const DEFAULT_STATE: InvaderState = {
  pose: 'idle',
  speech: null,
  eye: null,
  shake: false,
  jump: false,
};

interface InvaderContextValue {
  state: InvaderState;
  reactTo: (event: InvaderEvent) => void;
  setState: (state: Partial<InvaderState>) => void;
  resetState: () => void;
}

const InvaderCtx = createContext<InvaderContextValue | null>(null);

// Priority map for events — higher priority interrupts lower
const EVENT_PRIORITY: Record<string, number> = {
  tournament_started: 8,
  tournament_completed: 8,
  rank_up: 6,
  rank_down: 5,
  match_win: 5,
  match_loss: 4,
  match_draw: 3,
  program_uploaded: 3,
  program_error: 4,
  team_created: 4,
  invite_copied: 2,
  error_400: 3,
  error_403: 4,
  error_404: 3,
  error_500: 5,
};

function pickRandom<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)];
}

function getReaction(event: InvaderEvent): { poses: { pose: InvaderPose; duration: number }[]; speeches: string[]; eye: InvaderState['eye']; shake?: boolean; jump?: boolean } {
  switch (event.type) {
    case 'match_win':
      return {
        poses: [
          { pose: 'attack', duration: 600 },
          { pose: 'handsUp', duration: 800 },
          { pose: 'dance', duration: 1500 },
        ],
        speeches: ['>>> ПОБЕДА!', 'GG EZ', 'rm -rf opponent', '// доминация', 'exit(0)'],
        eye: 'wide',
        jump: true,
      };
    case 'match_loss':
      return {
        poses: [
          { pose: 'shield', duration: 600 },
          { pose: 'cry', duration: 2000 },
        ],
        speeches: ['segfault :(', 'exit(1)', '// в другой раз', 'core dumped', '// нужен рефакторинг'],
        eye: 'sad',
        shake: true,
      };
    case 'match_draw':
      return {
        poses: [
          { pose: 'idle', duration: 500 },
          { pose: 'handsUp', duration: 1500 },
        ],
        speeches: ['// ничья', 'draw()', '== ==', '// 50/50'],
        eye: null,
      };
    case 'rank_up':
      return {
        poses: [
          { pose: 'fly', duration: 1200 },
          { pose: 'dance', duration: 1500 },
        ],
        speeches: [`+${event.delta} позиций!`, 'level up!', '// вверх!', '>>> upgrade'],
        eye: 'wide',
        jump: true,
      };
    case 'rank_down':
      return {
        poses: [
          { pose: 'cry', duration: 2000 },
        ],
        speeches: [`-${event.delta} позиций...`, '// спуск', 'downgrade :('],
        eye: 'sad',
        shake: true,
      };
    case 'program_uploaded':
      return {
        poses: [
          { pose: 'handsUp', duration: 800 },
          { pose: 'dance', duration: 1200 },
        ],
        speeches: ['// код загружен!', 'compile OK', '$ upload success'],
        eye: 'wide',
        jump: true,
      };
    case 'program_error':
      return {
        poses: [
          { pose: 'cry', duration: 2000 },
        ],
        speeches: ['// ошибка компиляции', `Error: ${event.message.slice(0, 20)}`, 'syntax error!'],
        eye: 'sad',
        shake: true,
      };
    case 'tournament_started':
      return {
        poses: [
          { pose: 'teleport', duration: 800 },
          { pose: 'attack', duration: 800 },
          { pose: 'fly', duration: 1200 },
        ],
        speeches: ['>> СТАРТ <<', 'GLHF', '// турнир начался!', 'let battle = begin()'],
        eye: 'wide',
        jump: true,
      };
    case 'tournament_completed':
      return {
        poses: [
          { pose: 'fly', duration: 1000 },
          { pose: 'dance', duration: 2000 },
        ],
        speeches: ['// турнир завершён!', 'GG WP', 'tournament.finish()'],
        eye: 'wide',
        jump: true,
      };
    case 'team_created':
      return {
        poses: [
          { pose: 'handsUp', duration: 1000 },
          { pose: 'dance', duration: 1500 },
        ],
        speeches: [`// ${event.teamName}!`, 'new Team()', '// команда создана!'],
        eye: 'wide',
        jump: true,
      };
    case 'invite_copied':
      return {
        poses: [{ pose: 'handsUp', duration: 1500 }],
        speeches: ['// скопировано!', 'clipboard.write()', 'Ctrl+V'],
        eye: 'wide',
      };
    case 'error_400':
      return {
        poses: [{ pose: 'cry', duration: 2000 }],
        speeches: ['Bad Request!', '// 400', 'parse error'],
        eye: 'sad',
        shake: true,
      };
    case 'error_403':
      return {
        poses: [{ pose: 'shield', duration: 2000 }],
        speeches: ['// доступ запрещён', '403 Forbidden', 'permission denied'],
        eye: 'sad',
      };
    case 'error_404':
      return {
        poses: [{ pose: 'cry', duration: 2000 }],
        speeches: ['// 404', '// не найдено', 'null pointer'],
        eye: 'sad',
      };
    case 'error_500':
      return {
        poses: [{ pose: 'cry', duration: 2500 }],
        speeches: ['kernel panic!', 'core dumped', '// 500', 'FATAL ERROR'],
        eye: 'sad',
        shake: true,
      };
  }
}

export function InvaderProvider({ children }: { children: ReactNode }) {
  const [state, setFullState] = useState<InvaderState>(DEFAULT_STATE);
  const currentPriorityRef = useRef(0);
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  const clearTimers = useCallback(() => {
    timersRef.current.forEach(clearTimeout);
    timersRef.current = [];
  }, []);

  const resetState = useCallback(() => {
    clearTimers();
    currentPriorityRef.current = 0;
    setFullState(DEFAULT_STATE);
  }, [clearTimers]);

  const setState = useCallback((partial: Partial<InvaderState>) => {
    setFullState(prev => ({ ...prev, ...partial }));
  }, []);

  const reactTo = useCallback((event: InvaderEvent) => {
    const priority = EVENT_PRIORITY[event.type] || 1;
    if (priority < currentPriorityRef.current) return;

    clearTimers();
    currentPriorityRef.current = priority;

    const reaction = getReaction(event);
    const speech = pickRandom(reaction.speeches);

    // Set initial state
    setFullState({
      pose: reaction.poses[0].pose,
      speech,
      eye: reaction.eye,
      shake: reaction.shake ?? false,
      jump: reaction.jump ?? false,
    });

    // Clear shake/jump after a short duration
    const t0 = setTimeout(() => {
      setFullState(prev => ({ ...prev, shake: false, jump: false }));
    }, 600);
    timersRef.current.push(t0);

    // Sequence through poses
    let elapsed = reaction.poses[0].duration;
    for (let i = 1; i < reaction.poses.length; i++) {
      const p = reaction.poses[i];
      const t = setTimeout(() => {
        setFullState(prev => ({ ...prev, pose: p.pose }));
      }, elapsed);
      timersRef.current.push(t);
      elapsed += p.duration;
    }

    // Return to idle
    const tEnd = setTimeout(() => {
      currentPriorityRef.current = 0;
      setFullState(DEFAULT_STATE);
    }, elapsed);
    timersRef.current.push(tEnd);
  }, [clearTimers]);

  return (
    <InvaderCtx.Provider value={{ state, reactTo, setState, resetState }}>
      {children}
    </InvaderCtx.Provider>
  );
}

export function useInvaderContext() {
  const ctx = useContext(InvaderCtx);
  if (!ctx) throw new Error('useInvaderContext must be used within InvaderProvider');
  return ctx;
}
