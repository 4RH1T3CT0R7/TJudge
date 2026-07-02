import { useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { useToastStore } from '../../store/toastStore';
import { PlayIcon, PuzzlePieceIcon } from '../icons';
import { AutoRoundCountdown } from './AutoRoundCountdown';
import type {
  Team,
  Game,
  TournamentStatus,
  MatchRound,
  TournamentGameWithDetails,
} from '../../types';

// Games Tab Component
export function GamesTab({
  games,
  gamesStatus,
  tournamentId,
  myTeam,
  isAdmin,
  tournamentStatus,
  onRunGameMatches,
  onSetActiveGame,
  onResetGameRound,
  runningGameId,
  settingActiveGameId,
  resettingGameId,
  matchRounds,
}: {
  games: Game[];
  gamesStatus: TournamentGameWithDetails[];
  tournamentId: string;
  myTeam: Team | null;
  isAdmin?: boolean;
  tournamentStatus?: TournamentStatus;
  onRunGameMatches?: (gameId: string, gameName: string, gameDisplayName: string) => Promise<void>;
  onSetActiveGame?: (gameId: string) => Promise<void>;
  onResetGameRound?: (gameId: string, gameDisplayName: string) => Promise<void>;
  runningGameId?: string | null;
  settingActiveGameId?: string | null;
  resettingGameId?: string | null;
  matchRounds?: MatchRound[];
}) {
  const queryClient = useQueryClient();

  const handleRunMatches = async (e: React.MouseEvent, game: Game) => {
    e.preventDefault();
    e.stopPropagation();
    if (!onRunGameMatches) return;
    await onRunGameMatches(game.id, game.name, game.display_name);
  };

  const handleSetActive = async (e: React.MouseEvent, gameId: string) => {
    e.preventDefault();
    e.stopPropagation();
    if (!onSetActiveGame) return;
    await onSetActiveGame(gameId);
  };

  const handleReset = async (e: React.MouseEvent, game: Game) => {
    e.preventDefault();
    e.stopPropagation();
    if (!onResetGameRound) return;
    await onResetGameRound(game.id, game.display_name);
  };

  const handleToggleAutoRound = async (e: React.MouseEvent, gameId: string, currentStatus: TournamentGameWithDetails | undefined) => {
    e.preventDefault();
    e.stopPropagation();
    if (!tournamentId) return;

    const isEnabled = currentStatus?.auto_round_enabled ?? false;

    if (!isEnabled) {
      const intervalStr = window.prompt('Интервал авто-раунда (секунды, 10-3600):', '60');
      if (!intervalStr) return;
      const interval = parseInt(intervalStr, 10);
      if (isNaN(interval) || interval < 10 || interval > 3600) {
        useToastStore.getState().addToast('Интервал должен быть от 10 до 3600 секунд', 'error');
        return;
      }
      try {
        await api.setAutoRound(tournamentId, gameId, true, interval);
        await queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      } catch (err) {
        console.error('Failed to enable auto-round:', err);
        useToastStore.getState().addToast('Не удалось включить авто-раунд', 'error');
      }
    } else {
      try {
        await api.setAutoRound(tournamentId, gameId, false, currentStatus?.auto_round_interval_seconds ?? 60);
        await queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      } catch (err) {
        console.error('Failed to disable auto-round:', err);
        useToastStore.getState().addToast('Не удалось выключить авто-раунд', 'error');
      }
    }
  };

  if (games.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-state-icon">
          <PuzzlePieceIcon />
        </div>
        <h3 className="empty-state-title">Нет игр</h3>
        <p className="empty-state-description">
          В этот турнир еще не добавлены игры
        </p>
      </div>
    );
  }

  return (
    <div>
      {/* Admin info banner */}
      {isAdmin && tournamentStatus === 'active' && (
        <div className="mb-6 p-4 bg-blue-900/20 border border-blue-700 rounded-lg">
          <div className="flex items-start gap-3">
            <div className="shrink-0 w-8 h-8 bg-blue-800 rounded-lg flex items-center justify-center">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-blue-400">
                <path strokeLinecap="round" strokeLinejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />
              </svg>
            </div>
            <div>
              <h4 className="font-medium text-blue-200">Режим администратора</h4>
              <p className="text-sm text-blue-300 mt-1">
                Выберите активную игру и запустите раунд матчей. После запуска матчей команды не смогут изменять свои программы для этой игры.
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {games.map((game, index) => {
          const gameStatus = gamesStatus.find(g => g.game_id === game.id);
          const isActive = gameStatus?.is_active || false;
          const currentRound = gameStatus?.current_round || 0;
          const hasActiveMatches = (matchRounds || []).some(
            r => r.game_type === game.name && (r.pending_count > 0 || r.running_count > 0)
          );
          const roundInProgress = currentRound > 0 && gameStatus?.round_completed === false;
          const isRoundRunning = hasActiveMatches || roundInProgress;

          return (
            <Link
              key={game.id}
              to={`/tournaments/${tournamentId}/games/${game.id}`}
              className={`card card-interactive group relative overflow-hidden ${
                isActive ? 'ring-2 ring-green-600' : ''
              }`}
            >
              {/* Game number badge */}
              <div className="absolute top-3 right-3 w-8 h-8 bg-gray-800 rounded-full flex items-center justify-center text-sm font-bold text-gray-300">
                {index + 1}
              </div>

              <div className="mb-3 pr-10">
                <h3 className="text-lg font-bold text-gray-100 group-hover:text-primary-400 transition-colors">
                  {game.display_name}
                </h3>
                <div className="flex items-center gap-2 mt-1 flex-wrap">
                  <code className="text-sm bg-gray-800 px-2 py-0.5 rounded text-gray-400">
                    {game.name}
                  </code>
                  {currentRound > 0 && (
                    <span className="text-xs text-gray-400">
                      • Раунд {currentRound}
                    </span>
                  )}
                  {isActive && (
                    <span className="px-2 py-0.5 bg-green-900/50 text-green-400 text-xs rounded-full font-medium">
                      Активна
                    </span>
                  )}
                </div>
              </div>

              {game.rules && (
                <p className="text-gray-300 text-sm line-clamp-3 mb-4">
                  {game.rules.substring(0, 200)}...
                </p>
              )}

              {gameStatus?.auto_round_enabled && (
                <div className="mb-3">
                  <AutoRoundCountdown
                    enabled={gameStatus.auto_round_enabled}
                    intervalSeconds={gameStatus.auto_round_interval_seconds}
                    lastRunAt={gameStatus.auto_round_last_run_at}
                  />
                </div>
              )}

              <div className="flex items-center justify-between pt-3 border-t border-gray-700">
                {myTeam && !isAdmin && (
                  <div className="flex items-center gap-2 text-primary-400 text-sm font-medium">
                    <PlayIcon />
                    <span>Управление программой</span>
                  </div>
                )}

                {/* Admin controls */}
                {isAdmin && tournamentStatus === 'active' && (
                  <div className="flex flex-col gap-2 w-full">
                    {!isActive ? (
                      <button
                        onClick={(e) => handleSetActive(e, game.id)}
                        disabled={settingActiveGameId === game.id}
                        className="btn btn-secondary text-xs py-1.5 px-3"
                      >
                        {settingActiveGameId === game.id ? (
                          <>
                            <span className="w-3 h-3 border-2 border-gray-400/30 border-t-gray-600 rounded-full animate-spin" />
                            Установка...
                          </>
                        ) : (
                          'Сделать активной'
                        )}
                      </button>
                    ) : (
                      <div className="flex gap-2">
                        <button
                          onClick={(e) => handleRunMatches(e, game)}
                          disabled={runningGameId === game.id || isRoundRunning}
                          className="btn btn-primary text-xs py-1.5 px-3 flex-1"
                        >
                          {runningGameId === game.id ? (
                            <>
                              <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                              Запуск...
                            </>
                          ) : isRoundRunning ? (
                            <>
                              <span className="w-3 h-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                              Выполняется...
                            </>
                          ) : (
                            <>
                              <PlayIcon />
                              Запустить раунд
                            </>
                          )}
                        </button>
                        <button
                          onClick={(e) => handleReset(e, game)}
                          disabled={resettingGameId === game.id}
                          className="btn text-xs py-1.5 px-3 bg-red-600 hover:bg-red-700 text-white"
                          title="Сбросить раунд (удалить все матчи и рейтинги)"
                        >
                          {resettingGameId === game.id ? 'Сброс...' : 'Сбросить'}
                        </button>
                        <button
                          onClick={(e) => handleToggleAutoRound(e, game.id, gameStatus)}
                          className={`btn text-xs py-1.5 px-3 ${
                            gameStatus?.auto_round_enabled
                              ? 'bg-green-600 hover:bg-green-700 text-white'
                              : 'bg-gray-600 hover:bg-gray-700 text-gray-200'
                          }`}
                          title={gameStatus?.auto_round_enabled
                            ? `Авто-раунд: каждые ${gameStatus.auto_round_interval_seconds}с`
                            : 'Включить авто-раунд'
                          }
                        >
                          {gameStatus?.auto_round_enabled
                            ? `Авто ✓ (${gameStatus.auto_round_interval_seconds}с)`
                            : 'Авто'}
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}
