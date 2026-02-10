import { useCallback, useEffect, useRef } from 'react';
import { SpaceInvader } from './SpaceInvader';
import { QuestTerminal } from './quest/QuestTerminal';
import { QuestInvader } from './quest/QuestInvader';
import { MiniGameEngine } from './quest/MiniGames';
import { useQuestState } from '../hooks/useQuestState';

export function TerminalQuest() {
  const { state, dispatch } = useQuestState();
  const poseTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  // Auto-reset invader pose after 3 seconds
  useEffect(() => {
    if (state.invaderPose !== 'idle' && state.invaderPose !== 'fly' && state.invaderPose !== 'sleep') {
      clearTimeout(poseTimerRef.current);
      poseTimerRef.current = setTimeout(() => {
        dispatch({ type: 'RESET_INVADER' });
      }, 3000);
    }
    return () => clearTimeout(poseTimerRef.current);
  }, [state.invaderPose, dispatch]);

  // Clear speech bubble after 3 seconds
  useEffect(() => {
    if (state.invaderSpeech) {
      const t = setTimeout(() => {
        dispatch({ type: 'CLEAR_SPEECH' });
      }, 3000);
      return () => clearTimeout(t);
    }
  }, [state.invaderSpeech, dispatch]);

  const handleMiniGameEnd = useCallback((result: 'win' | 'lose') => {
    dispatch({ type: 'END_MINIGAME', result });
  }, [dispatch]);

  return (
    <div className="space-y-6">
      {/* Section header */}
      <div className="text-center">
        <h2 className="text-2xl font-bold text-gray-100 mb-2">
          Интерактивный терминал
        </h2>
        <p className="text-gray-400 text-sm">
          Управляйте инвейдером командами —{' '}
          <span className="text-primary-400 font-mono">invader.jump()</span>
        </p>
      </div>

      {/* Desktop: terminal + invader side by side */}
      <div className="hidden md:grid md:grid-cols-[1fr,auto] gap-6 items-start">
        <div>
          {/* Mini-game overlay inside terminal area */}
          {state.activeGame ? (
            <div className="bg-gray-900/80 border border-gray-800 rounded-xl overflow-hidden backdrop-blur-sm">
              <div className="flex items-center gap-2 px-4 py-2.5 bg-gray-900 border-b border-gray-800">
                <div className="flex gap-1.5">
                  <div className="w-3 h-3 rounded-full bg-red-500/80" />
                  <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
                  <div className="w-3 h-3 rounded-full bg-green-500/80" />
                </div>
                <span className="ml-3 text-xs text-gray-500 font-mono">mini-game</span>
              </div>
              <div className="p-4" style={{ minHeight: '340px' }}>
                <MiniGameEngine game={state.activeGame} onEnd={handleMiniGameEnd} />
              </div>
            </div>
          ) : (
            <QuestTerminal state={state} dispatch={dispatch} />
          )}
        </div>
        <QuestInvader state={state} />
      </div>

      {/* Mobile fallback */}
      <div className="md:hidden text-center py-8">
        <SpaceInvader size="sm" />
        <p className="text-gray-400 mt-3 font-mono text-sm">
          // откройте на компьютере для интерактивного квеста
        </p>
      </div>
    </div>
  );
}
