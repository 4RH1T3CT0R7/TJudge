import type { Dispatch, SetStateAction } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { useToastStore } from '../../store/toastStore';
import { MATCHES, confirmCompleteTournament, confirmDeleteTournament, confirmResetRound, confirmRunRound, count } from './confirmations';
import { getGameConfig } from '../../utils/gameConfig';
import { mdPreview } from '../../utils/markdown';
import { extractErrorMessage } from '../tournament/helpers';
import type { Game, Tournament, TournamentGameWithDetails } from '../../types';
import { StatusLabel } from '../ui/StatusLabel';
import { Spinner } from '../ui/Spinner';
import { EmptyState } from '../ui/EmptyState';
import { Modal } from '../ui/Modal';
import { Field } from '../ui/Field';
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

  const handleDeleteTournament = async (tournament: Tournament) => {
    setActionError(null);
    try {
      if (!(await confirmDeleteTournament(tournament))) return;
      setAdminReaction('cry', '// удаляем...', 2000);
      await api.deleteTournament(tournament.id);
      await queryClient.invalidateQueries({ queryKey: queryKeys.tournaments() });
    } catch (err: unknown) {
      console.error('Failed to delete tournament:', err);
      setActionError(extractErrorMessage(err, 'Не удалось удалить турнир'));
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
      setActionError(extractErrorMessage(err, 'Не удалось запустить турнир'));
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
      setActionError(extractErrorMessage(err, 'Не удалось установить активную игру'));
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
  const handleResetGameRound = async (game: Game) => {
    if (!managingTournamentId) return;

    setActionError(null);
    try {
      if (!(await confirmResetRound(managingTournamentId, game))) return;
    } catch (err: unknown) {
      setActionError(extractErrorMessage(err, 'Не удалось подготовить сброс раунда'));
      return;
    }

    setResettingGame(game.id);
    try {
      const result = await api.resetGameRound(managingTournamentId, game.id);

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
      setActionError(extractErrorMessage(err, 'Не удалось сбросить раунд'));
    } finally {
      setResettingGame(null);
    }
  };

  // Run matches for a specific game
  const handleRunGameMatches = async (gameType: string, gameName: string) => {
    if (!managingTournamentId) return;

    setActionError(null);
    try {
      if (!(await confirmRunRound(managingTournamentId, { name: gameType, display_name: gameName }))) return;
    } catch (err: unknown) {
      setActionError(extractErrorMessage(err, 'Не удалось подготовить запуск раунда'));
      return;
    }

    setRunningGameMatches(gameType);

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
      useToastStore.getState().addToast(`Запущено ${count(result.enqueued, MATCHES)} для "${gameName}"`, 'success');
    } catch (err: unknown) {
      console.error('Failed to run game matches:', err);
      setActionError(extractErrorMessage(err, 'Не удалось запустить матчи'));
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

          <Modal open={showTournamentForm} onClose={resetTournamentForm} closeOnBackdrop={false} title="Создать турнир" maxWidth="max-w-lg">
            <div className="space-y-4">
              <Field label="Название" required>
                {(control) => (
                  <input
                    {...control}
                    type="text"
                    name="tournamentName"
                    value={tournamentForm.name}
                    onChange={(e) =>
                      setTournamentForm({ ...tournamentForm, name: e.target.value })
                    }
                    className="input"
                    placeholder="Название турнира"
                  />
                )}
              </Field>

              <fieldset className="min-w-0">
                <legend className="block text-sm font-medium mb-2 text-gray-300">
                  Игры турнира<span aria-hidden="true"> *</span><span className="sr-only">, обязательно</span>
                </legend>
                {games.length === 0 ? (
                  <p className="text-sm text-gray-400">
                    Сначала создайте игры во вкладке "Игры"
                  </p>
                ) : (
                  <div className="space-y-3">
                    {/* Available games */}
                    <div className="space-y-2 max-h-32 overflow-y-auto border border-line rounded-lg p-3 bg-gray-700">
                      {games.map((game) => (
                        <label
                          key={game.id}
                          className="flex items-center gap-3 p-2 hover:bg-line/50 rounded cursor-pointer"
                            >
                          <input
                            type="checkbox"
                            checked={selectedGameIds.includes(game.id)}
                            onChange={() => toggleGameSelection(game.id)}
                            className="w-4 h-4 text-primary-600 rounded"
                          />
                          <div>
                            <span className="font-medium text-gray-100">{game.display_name}</span>
                            <span className="text-xs text-gray-300 ml-2">({game.name})</span>
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
                                  <span aria-hidden="true" className={`inline-block w-6 shrink-0 text-center font-mono text-lg ${getGameConfig(game.name).textClass}`}>{getGameConfig(game.name).icon}</span>
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
                                    aria-label={`Переместить «${game.display_name}» вверх`}
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
                                    aria-label={`Переместить «${game.display_name}» вниз`}
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
              </fieldset>

              <Field label="Описание">
                {(control) => (
                  <textarea
                    {...control}
                    value={tournamentForm.description}
                    onChange={(e) =>
                      setTournamentForm({ ...tournamentForm, description: e.target.value })
                    }
                    className="input min-h-[100px]"
                    placeholder="Описание турнира..."
                  />
                )}
              </Field>

              <div className="grid grid-cols-2 gap-4">
                <Field label="Макс. размер команды">
                  {(control) => (
                    <input
                      {...control}
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
                  )}
                </Field>

                <Field label="Макс. участников">
                  {(control) => (
                    <input
                      {...control}
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
                  )}
                </Field>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <Field label="Дата начала">
                  {(control) => (
                    <input
                      {...control}
                      type="datetime-local"
                      value={tournamentForm.start_time}
                      onChange={(e) =>
                        setTournamentForm({ ...tournamentForm, start_time: e.target.value })
                      }
                      className="input"
                    />
                  )}
                </Field>

                <Field label="Дата окончания">
                  {(control) => (
                    <input
                      {...control}
                      type="datetime-local"
                      value={tournamentForm.end_time}
                      onChange={(e) =>
                        setTournamentForm({ ...tournamentForm, end_time: e.target.value })
                      }
                      className="input"
                    />
                  )}
                </Field>
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
                <div role="alert" className="p-2 bg-red-900/30 border border-red-800 rounded text-sm text-red-400">
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
          </Modal>

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
          <Modal
            open={managingTournamentId !== null}
            onClose={closeTournamentGamesManagement}
            title="Управление играми турнира"
            maxWidth="max-w-lg"
          >
            <p className="text-sm text-gray-400 mb-4">
              Выберите активную игру. Только активная игра может принимать загрузку программ.
              Кнопка «Запустить раунд» запустит матчи только для активной игры.
            </p>

            {isLoadingTournamentGames && !showLoadingTournamentGames ? (
              null
            ) : showLoadingTournamentGames ? (
              <div className="flex justify-center py-8 text-sm text-gray-400">
                <Spinner>загрузка игр</Spinner>
              </div>
            ) : managingTournamentGames.length === 0 ? (
              <EmptyState command="игры" hint="в этом турнире нет игр" />
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
                        <span aria-hidden="true" className={`inline-block w-8 shrink-0 text-center font-mono text-2xl ${getGameConfig(game.name).textClass}`}>{getGameConfig(game.name).icon}</span>
                        <div>
                          <p className="font-medium text-gray-100">
                            {game.display_name}
                          </p>
                          <div className="flex items-center gap-2 mt-0.5">
                            <span className="text-xs text-gray-400">
                              {game.name}
                            </span>
                            {isActive && (
                              <span className="px-2 py-0.5 bg-green-900/50 text-green-400 text-xs rounded font-medium">
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
                            className="btn btn-secondary"
                              >
                            {settingActiveGame === game.id ? 'Установка...' : 'Сделать активной'}
                          </button>
                        )}
                        {isActive && (
                          <>
                            <button
                              onClick={() => handleRunGameMatches(game.name, game.display_name)}
                              disabled={runningGameMatches === game.name}
                              className="btn btn-primary flex-1"
                                >
                              {runningGameMatches === game.name ? 'Запуск...' : 'Запустить раунд'}
                            </button>
                            <div className="flex border-l border-gray-700 pl-2">
                              <button
                                onClick={() => handleResetGameRound(game)}
                                disabled={resettingGame === game.id}
                                className="btn btn-danger"
                                title="Сбросить раунд (удалить все матчи и рейтинги)"
                              >
                                {resettingGame === game.id ? 'Сброс...' : 'Сбросить'}
                              </button>
                            </div>
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
          </Modal>

          {/* Tournaments List */}
          {tournaments.length === 0 ? (
            <EmptyState command="турниры" hint="турниры ещё не созданы" />
          ) : (
            <div className="space-y-4">
              {tournaments.map((tournament) => (
                <div key={tournament.id} className="card">
                  <div className="flex justify-between items-start">
                    <div>
                      <h3 className="font-semibold text-gray-100">{tournament.name}</h3>
                      <p className="text-sm text-gray-400">
                        Код турнира: <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{tournament.code}</code>
                      </p>
                      {tournament.description && (
                        <p className="text-sm text-gray-300 mt-1 line-clamp-2">
                          {mdPreview(tournament.description)}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <StatusLabel entity="tournament" status={tournament.status} />
                      {tournament.is_permanent && <span className="badge badge-blue">Постоянный</span>}
                    </div>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <a
                      href={`/tournaments/${tournament.id}`}
                      className="btn btn-secondary"
                    >
                      Просмотр
                    </a>
                    {tournament.status === 'pending' && (
                      <button
                        onClick={() => handleStartTournament(tournament.id)}
                        className="btn btn-primary"
                      >
                        Запустить
                      </button>
                    )}
                    {tournament.status === 'active' && (
                      <button
                        onClick={() => openTournamentGamesManagement(tournament.id)}
                        className="btn btn-primary"
                      >
                        Запустить раунд
                      </button>
                    )}
                    {/* необратимое действие отодвинуто вправо от остальных */}
                    <div className="ml-auto flex">
                      {tournament.status === 'active' ? (
                        <button
                          onClick={async () => {
                            setActionError(null);
                            try {
                              if (!(await confirmCompleteTournament(tournament))) return;
                              await api.completeTournament(tournament.id);
                              await Promise.all([
                                queryClient.invalidateQueries({ queryKey: queryKeys.tournaments() }),
                                queryClient.invalidateQueries({ queryKey: queryKeys.tournament(tournament.id) }),
                              ]);
                              setAdminReaction('salute', '// турнир окончен', 3000);
                            } catch (err: unknown) {
                              console.error('Failed to complete tournament:', err);
                              setActionError(extractErrorMessage(err, 'Не удалось завершить турнир'));
                            }
                          }}
                          className="btn btn-danger"
                        >
                          Завершить
                        </button>
                      ) : (
                        <button
                          onClick={() => handleDeleteTournament(tournament)}
                          className="btn btn-danger"
                        >
                          Удалить
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
  );
}
