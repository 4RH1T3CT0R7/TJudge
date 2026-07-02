import type { Dispatch, SetStateAction } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { useToastStore } from '../../store/toastStore';
import { confirmDialog } from '../../store/confirmStore';
import { getGameConfig } from '../../utils/gameConfig';
import type { Game, Tournament, TournamentGameWithDetails } from '../../types';
import { statusLabels } from './types';
import type { AdminReactionSetter, TournamentFormState } from './types';

interface TournamentsTabProps {
  tournaments: Tournament[];
  games: Game[];
  showTournamentForm: boolean;
  setShowTournamentForm: Dispatch<SetStateAction<boolean>>;
  tournamentForm: TournamentFormState;
  setTournamentForm: Dispatch<SetStateAction<TournamentFormState>>;
  selectedGameIds: string[];
  setSelectedGameIds: Dispatch<SetStateAction<string[]>>;
  isSavingTournament: boolean;
  setIsSavingTournament: Dispatch<SetStateAction<boolean>>;
  tournamentError: string | null;
  setTournamentError: Dispatch<SetStateAction<string | null>>;
  deleteTournamentId: string | null;
  setDeleteTournamentId: Dispatch<SetStateAction<string | null>>;
  actionError: string | null;
  setActionError: Dispatch<SetStateAction<string | null>>;
  managingTournamentId: string | null;
  setManagingTournamentId: Dispatch<SetStateAction<string | null>>;
  managingTournamentGames: Game[];
  managingTournamentGamesStatus: TournamentGameWithDetails[];
  isLoadingTournamentGames: boolean;
  showLoadingTournamentGames: boolean;
  runningGameMatches: string | null;
  setRunningGameMatches: Dispatch<SetStateAction<string | null>>;
  settingActiveGame: string | null;
  setSettingActiveGame: Dispatch<SetStateAction<string | null>>;
  resettingGame: string | null;
  setResettingGame: Dispatch<SetStateAction<string | null>>;
  setAdminReaction: AdminReactionSetter;
}

export function TournamentsTab({
  tournaments,
  games,
  showTournamentForm,
  setShowTournamentForm,
  tournamentForm,
  setTournamentForm,
  selectedGameIds,
  setSelectedGameIds,
  isSavingTournament,
  setIsSavingTournament,
  tournamentError,
  setTournamentError,
  deleteTournamentId,
  setDeleteTournamentId,
  actionError,
  setActionError,
  managingTournamentId,
  setManagingTournamentId,
  managingTournamentGames,
  managingTournamentGamesStatus,
  isLoadingTournamentGames,
  showLoadingTournamentGames,
  runningGameMatches,
  setRunningGameMatches,
  settingActiveGame,
  setSettingActiveGame,
  resettingGame,
  setResettingGame,
  setAdminReaction,
}: TournamentsTabProps) {
  const queryClient = useQueryClient();

  const handleDeleteTournament = async (id: string) => {
    setAdminReaction('cry', '// удаляем...', 2000);
    try {
      await api.deleteTournament(id);
      await queryClient.invalidateQueries({ queryKey: queryKeys.tournaments() });
      setDeleteTournamentId(null);
      setActionError(null);
    } catch (err: unknown) {
      console.error('Failed to delete tournament:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось удалить турнир');
    }
  };

  const handleStartTournament = async (id: string) => {
    setActionError(null);
    setAdminReaction('fly', '// запуск!', 3000);
    try {
      await api.startTournament(id);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.tournaments() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tournament(id) }),
      ]);
    } catch (err: unknown) {
      console.error('Failed to start tournament:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      const message = axiosErr.response?.data?.message || 'Не удалось запустить турнир';
      setActionError(message);
    }
  };

  const handleCreateTournament = async () => {
    if (!tournamentForm.name.trim()) {
      setTournamentError('Название обязательно');
      return;
    }
    if (selectedGameIds.length === 0) {
      setTournamentError('Выберите хотя бы одну игру');
      return;
    }

    setIsSavingTournament(true);
    setTournamentError(null);

    try {
      // Используем первую игру как game_type для совместимости
      const firstGame = games.find(g => g.id === selectedGameIds[0]);
      const payload: Record<string, unknown> = {
        name: tournamentForm.name,
        game_type: firstGame?.name || 'default',
        description: tournamentForm.description || undefined,
        max_team_size: tournamentForm.max_team_size,
        is_permanent: tournamentForm.is_permanent,
      };

      // Add optional fields
      if (tournamentForm.max_participants) {
        payload.max_participants = parseInt(tournamentForm.max_participants, 10);
      }
      if (tournamentForm.start_time) {
        payload.start_time = new Date(tournamentForm.start_time).toISOString();
      }

      const newTournament = await api.createTournament(payload);

      // Добавляем выбранные игры в турнир
      for (const gameId of selectedGameIds) {
        try {
          await api.addGameToTournament(newTournament.id, gameId);
        } catch (err) {
          console.error(`Failed to add game ${gameId} to tournament:`, err);
        }
      }

      await queryClient.invalidateQueries({ queryKey: queryKeys.tournaments() });
      resetTournamentForm();
    } catch (err) {
      console.error('Failed to create tournament:', err);
      setTournamentError('Не удалось создать турнир');
    } finally {
      setIsSavingTournament(false);
    }
  };

  const resetTournamentForm = () => {
    setShowTournamentForm(false);
    setTournamentForm({
      name: '',
      description: '',
      game_type: '',
      max_team_size: 3,
      max_participants: '',
      is_permanent: false,
      start_time: '',
      end_time: '',
    });
    setSelectedGameIds([]);
    setTournamentError(null);
  };

  const toggleGameSelection = (gameId: string) => {
    setSelectedGameIds(prev =>
      prev.includes(gameId)
        ? prev.filter(id => id !== gameId)
        : [...prev, gameId]
    );
  };

  // Move game up in the order
  const moveGameUp = (index: number) => {
    if (index <= 0) return;
    setSelectedGameIds(prev => {
      const newIds = [...prev];
      [newIds[index - 1], newIds[index]] = [newIds[index], newIds[index - 1]];
      return newIds;
    });
  };

  // Move game down in the order
  const moveGameDown = (index: number) => {
    if (index >= selectedGameIds.length - 1) return;
    setSelectedGameIds(prev => {
      const newIds = [...prev];
      [newIds[index], newIds[index + 1]] = [newIds[index + 1], newIds[index]];
      return newIds;
    });
  };

  // Open tournament games management modal (данные подтянут query-хуки по managingTournamentId)
  const openTournamentGamesManagement = (tournamentId: string) => {
    setManagingTournamentId(tournamentId);
  };

  // Close tournament games management modal
  const closeTournamentGamesManagement = () => {
    setManagingTournamentId(null);
    setRunningGameMatches(null);
    setSettingActiveGame(null);
  };

  // Set active game for tournament
  const handleSetActiveGame = async (gameId: string) => {
    if (!managingTournamentId) return;

    setSettingActiveGame(gameId);
    setActionError(null);

    try {
      await api.setActiveGame(managingTournamentId, gameId);
      // Reload games status to update UI
      await queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(managingTournamentId) });
    } catch (err: unknown) {
      console.error('Failed to set active game:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось установить активную игру');
    } finally {
      setSettingActiveGame(null);
    }
  };

  // Run round for active game only
  const handleRunActiveGameRound = async () => {
    if (!managingTournamentId) return;

    const activeGame = managingTournamentGamesStatus.find(g => g.is_active);
    if (!activeGame) {
      setActionError('Нет активной игры. Выберите активную игру для запуска раунда.');
      return;
    }

    const game = managingTournamentGames.find(g => g.id === activeGame.game_id);
    if (!game) return;

    await handleRunGameMatches(game.name, game.display_name);
  };

  // Reset game round (delete all matches and reset ratings)
  const handleResetGameRound = async (gameId: string, gameName: string) => {
    if (!managingTournamentId) return;

    const confirmed = await confirmDialog({
      title: 'Сброс раунда',
      message:
        `Сбросить раунд для игры "${gameName}"?\n\n` +
        'Это действие:\n' +
        '- удалит все матчи этой игры\n' +
        '- сбросит рейтинги всех участников до 1000\n' +
        '- сбросит номер раунда\n\n' +
        'Это действие необратимо!',
      confirmLabel: 'Сбросить',
      danger: true,
    });

    if (!confirmed) return;

    setResettingGame(gameId);
    setActionError(null);

    try {
      const result = await api.resetGameRound(managingTournamentId, gameId);

      // Сбрасываем всё поддерево турнира (статусы игр, лидерборды, программы) и статистику матчей
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.tournament(managingTournamentId) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.matchStatistics() }),
      ]);

      useToastStore.getState().addToast(
        `Раунд сброшен: матчей удалено ${result.matches_deleted}, рейтингов сброшено ${result.participants_reset}`,
        'success',
        8000
      );
    } catch (err: unknown) {
      console.error('Failed to reset game round:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось сбросить раунд');
    } finally {
      setResettingGame(null);
    }
  };

  // Run matches for a specific game
  const handleRunGameMatches = async (gameType: string, gameName: string) => {
    if (!managingTournamentId) return;

    setRunningGameMatches(gameType);
    setActionError(null);

    try {
      const result = await api.runGameMatches(managingTournamentId, gameType);
      setActionError(null);
      // Очередь и статусы игр изменились — инвалидируем связанные ключи
      void Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.queueStats }),
        queryClient.invalidateQueries({ queryKey: queryKeys.matchStatistics() }),
        queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(managingTournamentId) }),
      ]);
      // Show success message
      useToastStore.getState().addToast(`Запущено ${result.enqueued} матчей для "${gameName}"`, 'success');
    } catch (err: unknown) {
      console.error('Failed to run game matches:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось запустить матчи');
    } finally {
      setRunningGameMatches(null);
    }
  };

  return (
        <div>
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-semibold text-gray-100">Управление турнирами</h2>
            <button onClick={() => { setShowTournamentForm(true); setAdminReaction('typing', '// создаём турнир?', 2500); }} className="btn btn-primary">
              Создать турнир
            </button>
          </div>

          {/* Tournament Form Modal */}
          {showTournamentForm && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-gray-800 rounded-lg p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto">
                <h2 className="text-xl font-bold mb-4 text-gray-100">Создать турнир</h2>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-300">Название *</label>
                    <input
                      type="text"
                      name="tournamentName"
                      value={tournamentForm.name}
                      onChange={(e) =>
                        setTournamentForm({ ...tournamentForm, name: e.target.value })
                      }
                      className="input"
                      placeholder="Название турнира"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-2 text-gray-300">Игры турнира *</label>
                    {games.length === 0 ? (
                      <p className="text-sm text-gray-400">
                        Сначала создайте игры во вкладке "Игры"
                      </p>
                    ) : (
                      <div className="space-y-3">
                        {/* Available games */}
                        <div className="space-y-2 max-h-32 overflow-y-auto border border-gray-600 rounded-lg p-3 bg-gray-700">
                          {games.map((game) => (
                            <label
                              key={game.id}
                              className="flex items-center gap-3 p-2 hover:bg-gray-600 rounded cursor-pointer"
                            >
                              <input
                                type="checkbox"
                                checked={selectedGameIds.includes(game.id)}
                                onChange={() => toggleGameSelection(game.id)}
                                className="w-4 h-4 text-primary-600 rounded"
                              />
                              <div>
                                <span className="font-medium text-gray-100">{game.display_name}</span>
                                <span className="text-xs text-gray-400 ml-2">({game.name})</span>
                              </div>
                            </label>
                          ))}
                        </div>

                        {/* Selected games with order controls */}
                        {selectedGameIds.length > 0 && (
                          <div>
                            <p className="text-sm font-medium text-gray-300 mb-2">
                              Порядок игр (раунды будут запускаться в этом порядке):
                            </p>
                            <div className="space-y-2 border border-primary-800 rounded-lg p-3 bg-primary-900/20">
                              {selectedGameIds.map((gameId, index) => {
                                const game = games.find(g => g.id === gameId);
                                if (!game) return null;
                                return (
                                  <div
                                    key={gameId}
                                    className="flex items-center justify-between p-2 bg-gray-800 rounded border border-gray-700"
                                  >
                                    <div className="flex items-center gap-2">
                                      <span className="text-sm font-bold text-primary-400 w-6">
                                        {index + 1}.
                                      </span>
                                      <span className="text-lg">{getGameConfig(game.name).icon}</span>
                                      <span className="font-medium text-gray-100">
                                        {game.display_name}
                                      </span>
                                    </div>
                                    <div className="flex items-center gap-1">
                                      <button
                                        type="button"
                                        onClick={() => moveGameUp(index)}
                                        disabled={index === 0}
                                        className="p-1 text-gray-400 hover:text-gray-200 disabled:opacity-30"
                                        title="Вверх"
                                      >
                                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-4 h-4">
                                          <path strokeLinecap="round" strokeLinejoin="round" d="m4.5 15.75 7.5-7.5 7.5 7.5" />
                                        </svg>
                                      </button>
                                      <button
                                        type="button"
                                        onClick={() => moveGameDown(index)}
                                        disabled={index === selectedGameIds.length - 1}
                                        className="p-1 text-gray-400 hover:text-gray-200 disabled:opacity-30"
                                        title="Вниз"
                                      >
                                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-4 h-4">
                                          <path strokeLinecap="round" strokeLinejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
                                        </svg>
                                      </button>
                                    </div>
                                  </div>
                                );
                              })}
                            </div>
                          </div>
                        )}
                      </div>
                    )}
                    {selectedGameIds.length > 0 && (
                      <p className="text-xs text-gray-400 mt-1">
                        Выбрано игр: {selectedGameIds.length}
                      </p>
                    )}
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-300">Описание</label>
                    <textarea
                      value={tournamentForm.description}
                      onChange={(e) =>
                        setTournamentForm({ ...tournamentForm, description: e.target.value })
                      }
                      className="input min-h-[100px]"
                      placeholder="Описание турнира..."
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium mb-1 text-gray-300">Макс. размер команды</label>
                      <input
                        type="number"
                        value={tournamentForm.max_team_size}
                        onChange={(e) =>
                          setTournamentForm({
                            ...tournamentForm,
                            max_team_size: parseInt(e.target.value) || 1,
                          })
                        }
                        className="input"
                        min={1}
                        max={10}
                      />
                    </div>

                    <div>
                      <label className="block text-sm font-medium mb-1 text-gray-300">Макс. участников</label>
                      <input
                        type="number"
                        value={tournamentForm.max_participants}
                        onChange={(e) =>
                          setTournamentForm({
                            ...tournamentForm,
                            max_participants: e.target.value,
                          })
                        }
                        className="input"
                        min={2}
                        placeholder="Без ограничений"
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium mb-1 text-gray-300">Дата начала</label>
                      <input
                        type="datetime-local"
                        value={tournamentForm.start_time}
                        onChange={(e) =>
                          setTournamentForm({ ...tournamentForm, start_time: e.target.value })
                        }
                        className="input"
                      />
                    </div>

                    <div>
                      <label className="block text-sm font-medium mb-1 text-gray-300">Дата окончания</label>
                      <input
                        type="datetime-local"
                        value={tournamentForm.end_time}
                        onChange={(e) =>
                          setTournamentForm({ ...tournamentForm, end_time: e.target.value })
                        }
                        className="input"
                      />
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="is_permanent"
                      checked={tournamentForm.is_permanent}
                      onChange={(e) =>
                        setTournamentForm({
                          ...tournamentForm,
                          is_permanent: e.target.checked,
                        })
                      }
                      className="w-4 h-4"
                    />
                    <label htmlFor="is_permanent" className="text-sm text-gray-300">
                      Постоянный турнир (всегда принимает новых участников)
                    </label>
                  </div>

                  {tournamentError && (
                    <div className="p-2 bg-red-900/30 border border-red-800 rounded text-sm text-red-400">
                      {tournamentError}
                    </div>
                  )}
                </div>

                <div className="flex justify-end gap-2 mt-6">
                  <button onClick={resetTournamentForm} className="btn btn-secondary">
                    Отмена
                  </button>
                  <button
                    onClick={handleCreateTournament}
                    disabled={isSavingTournament}
                    className="btn btn-primary"
                  >
                    {isSavingTournament ? 'Создание...' : 'Создать'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Action Error */}
          {actionError && (
            <div className="mb-4 p-3 bg-red-900/30 border border-red-800 rounded-lg text-sm text-red-400">
              {actionError}
              <button
                onClick={() => setActionError(null)}
                className="ml-2 text-red-400 hover:text-red-300"
              >
                ✕
              </button>
            </div>
          )}

          {/* Tournament Games Management Modal */}
          {managingTournamentId && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-gray-800 rounded-lg p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto">
                <div className="flex justify-between items-center mb-4">
                  <h2 className="text-xl font-bold text-gray-100">
                    Управление играми турнира
                  </h2>
                  <button
                    onClick={closeTournamentGamesManagement}
                    aria-label="Закрыть"
                    className="text-gray-400 hover:text-gray-300"
                  >
                    ✕
                  </button>
                </div>

                <p className="text-sm text-gray-400 mb-4">
                  Выберите активную игру. Только активная игра может принимать загрузку программ.
                  Кнопка «Запустить раунд» запустит матчи только для активной игры.
                </p>

                {isLoadingTournamentGames && !showLoadingTournamentGames ? (
                  null
                ) : showLoadingTournamentGames ? (
                  <div className="text-center py-8 text-gray-400">
                    Загрузка игр...
                  </div>
                ) : managingTournamentGames.length === 0 ? (
                  <div className="text-center py-8 text-gray-400">
                    В этом турнире нет игр
                  </div>
                ) : (
                  <div className="space-y-3">
                    {managingTournamentGames.map((game) => {
                      const gameStatus = managingTournamentGamesStatus.find(g => g.game_id === game.id);
                      const isActive = gameStatus?.is_active || false;
                      return (
                        <div
                          key={game.id}
                          className={`p-3 border rounded-lg transition-colors ${
                            isActive
                              ? 'border-green-600 bg-green-900/20'
                              : 'border-gray-700'
                          }`}
                        >
                          <div className="flex items-center gap-3 mb-2">
                            <span className="text-2xl">{getGameConfig(game.name).icon}</span>
                            <div>
                              <p className="font-medium text-gray-100">
                                {game.display_name}
                              </p>
                              <div className="flex items-center gap-2 mt-0.5">
                                <span className="text-xs text-gray-400">
                                  {game.name}
                                  {gameStatus && ` • Раунд ${gameStatus.current_round}`}
                                </span>
                                {isActive && (
                                  <span className="px-2 py-0.5 bg-green-900/50 text-green-400 text-xs rounded-full font-medium">
                                    Активна
                                  </span>
                                )}
                              </div>
                            </div>
                          </div>
                          <div className="flex gap-2">
                            {!isActive && (
                              <button
                                onClick={() => handleSetActiveGame(game.id)}
                                disabled={settingActiveGame === game.id}
                                className="btn btn-secondary text-sm disabled:opacity-50"
                              >
                                {settingActiveGame === game.id ? 'Установка...' : 'Сделать активной'}
                              </button>
                            )}
                            {isActive && (
                              <>
                                <button
                                  onClick={() => handleRunGameMatches(game.name, game.display_name)}
                                  disabled={runningGameMatches === game.name}
                                  className="btn btn-primary text-sm disabled:opacity-50 flex-1"
                                >
                                  {runningGameMatches === game.name ? 'Запуск...' : 'Запустить раунд'}
                                </button>
                                <button
                                  onClick={() => handleResetGameRound(game.id, game.display_name)}
                                  disabled={resettingGame === game.id}
                                  className="btn text-sm bg-red-600 hover:bg-red-700 text-white disabled:opacity-50"
                                  title="Сбросить раунд (удалить все матчи и рейтинги)"
                                >
                                  {resettingGame === game.id ? 'Сброс...' : 'Сбросить'}
                                </button>
                              </>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}

                <div className="flex justify-between mt-6">
                  <button onClick={closeTournamentGamesManagement} className="btn btn-secondary">
                    Закрыть
                  </button>
                  {managingTournamentGamesStatus.some(g => g.is_active) && (
                    <button
                      onClick={handleRunActiveGameRound}
                      disabled={runningGameMatches !== null}
                      className="btn btn-primary"
                    >
                      {runningGameMatches ? 'Запуск...' : 'Запустить раунд активной игры'}
                    </button>
                  )}
                </div>
              </div>
            </div>
          )}

          {/* Tournaments List */}
          {tournaments.length === 0 ? (
            <div className="text-center py-8 text-gray-400 bg-gray-800 rounded-lg">
              Турниры ещё не созданы.
            </div>
          ) : (
            <div className="space-y-4">
              {tournaments.map((tournament) => (
                <div key={tournament.id} className="card">
                  <div className="flex justify-between items-start">
                    <div>
                      <h3 className="font-semibold text-gray-100">{tournament.name}</h3>
                      <p className="text-sm text-gray-400">
                        Код: <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{tournament.code}</code>
                      </p>
                      {tournament.description && (
                        <p className="text-sm text-gray-300 mt-1 line-clamp-2">
                          {tournament.description}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <span
                        className={`px-2 py-1 rounded text-xs font-medium ${
                          tournament.status === 'pending'
                            ? 'bg-yellow-900/50 text-yellow-300'
                            : tournament.status === 'active'
                            ? 'bg-green-900/50 text-green-300'
                            : 'bg-gray-700 text-gray-300'
                        }`}
                      >
                        {statusLabels[tournament.status]}
                      </span>
                      {tournament.is_permanent && (
                        <span className="bg-blue-900/50 text-blue-300 px-2 py-1 rounded text-xs font-medium">
                          Постоянный
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <a
                      href={`/tournaments/${tournament.id}`}
                      className="btn btn-secondary text-sm"
                    >
                      Просмотр
                    </a>
                    {tournament.status === 'pending' && (
                      <button
                        onClick={() => handleStartTournament(tournament.id)}
                        className="btn btn-primary text-sm"
                      >
                        Запустить
                      </button>
                    )}
                    {tournament.status === 'active' && (
                      <>
                        <button
                          onClick={() => openTournamentGamesManagement(tournament.id)}
                          className="btn btn-primary text-sm"
                        >
                          Запустить раунд
                        </button>
                        <button
                          onClick={async () => {
                            setActionError(null);
                            try {
                              await api.completeTournament(tournament.id);
                              await Promise.all([
                                queryClient.invalidateQueries({ queryKey: queryKeys.tournaments() }),
                                queryClient.invalidateQueries({ queryKey: queryKeys.tournament(tournament.id) }),
                              ]);
                              setAdminReaction('salute', '// турнир окончен', 3000);
                            } catch (err: unknown) {
                              console.error('Failed to complete tournament:', err);
                              const axiosErr = err as { response?: { data?: { message?: string } } };
                              setActionError(axiosErr.response?.data?.message || 'Не удалось завершить турнир');
                            }
                          }}
                          className="btn btn-secondary text-sm"
                        >
                          Завершить
                        </button>
                      </>
                    )}
                    {tournament.status !== 'active' && (
                      <>
                        {deleteTournamentId === tournament.id ? (
                          <div className="flex gap-1">
                            <button
                              onClick={() => handleDeleteTournament(tournament.id)}
                              className="btn btn-danger text-sm"
                            >
                              Подтвердить
                            </button>
                            <button
                              onClick={() => setDeleteTournamentId(null)}
                              className="btn btn-secondary text-sm"
                            >
                              Отмена
                            </button>
                          </div>
                        ) : (
                          <button
                            onClick={() => setDeleteTournamentId(tournament.id)}
                            className="btn btn-danger text-sm"
                          >
                            Удалить
                          </button>
                        )}
                      </>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
  );
}
