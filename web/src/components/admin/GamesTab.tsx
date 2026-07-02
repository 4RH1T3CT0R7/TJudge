import type { Dispatch, SetStateAction } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import type { Game } from '../../types';
import type { AdminReactionSetter, GameFormState } from './types';

interface GamesTabProps {
  games: Game[];
  showGameForm: boolean;
  setShowGameForm: Dispatch<SetStateAction<boolean>>;
  editingGame: Game | null;
  setEditingGame: Dispatch<SetStateAction<Game | null>>;
  gameForm: GameFormState;
  setGameForm: Dispatch<SetStateAction<GameFormState>>;
  isSavingGame: boolean;
  setIsSavingGame: Dispatch<SetStateAction<boolean>>;
  gameError: string | null;
  setGameError: Dispatch<SetStateAction<string | null>>;
  deleteGameId: string | null;
  setDeleteGameId: Dispatch<SetStateAction<string | null>>;
  setAdminReaction: AdminReactionSetter;
}

export function GamesTab({
  games,
  showGameForm,
  setShowGameForm,
  editingGame,
  setEditingGame,
  gameForm,
  setGameForm,
  isSavingGame,
  setIsSavingGame,
  gameError,
  setGameError,
  deleteGameId,
  setDeleteGameId,
  setAdminReaction,
}: GamesTabProps) {
  const queryClient = useQueryClient();

  const handleCreateGame = async () => {
    if (!gameForm.name.trim() || !gameForm.display_name.trim()) {
      setGameError('Название и отображаемое имя обязательны');
      return;
    }

    // Validate name format
    if (!/^[a-z0-9_]+$/.test(gameForm.name)) {
      setGameError('Название должно содержать только строчные буквы, цифры и подчёркивания');
      return;
    }

    setIsSavingGame(true);
    setGameError(null);
    setAdminReaction('typing', '// создаём...', 2000);

    try {
      if (editingGame) {
        await api.updateGame(editingGame.id, {
          display_name: gameForm.display_name,
          rules: gameForm.rules,
        });
      } else {
        await api.createGame(gameForm);
      }
      // Ключ queryKeys.games — префикс и для queryKeys.game(id), инвалидируются оба
      await queryClient.invalidateQueries({ queryKey: queryKeys.games });
      setAdminReaction('celebrate', '// готово!', 2000);
      resetGameForm();
    } catch (err) {
      console.error('Failed to save game:', err);
      setGameError('Не удалось сохранить игру');
    } finally {
      setIsSavingGame(false);
    }
  };

  const handleDeleteGame = async (id: string) => {
    setAdminReaction('cry', '// удаляем...', 2000);
    try {
      await api.deleteGame(id);
      await queryClient.invalidateQueries({ queryKey: queryKeys.games });
      setDeleteGameId(null);
    } catch (err) {
      console.error('Failed to delete game:', err);
      setAdminReaction('dizzy', '// ошибка!', 2000);
    }
  };

  const resetGameForm = () => {
    setShowGameForm(false);
    setEditingGame(null);
    setGameForm({ name: '', display_name: '', rules: '' });
    setGameError(null);
  };

  const startEditGame = (game: Game) => {
    setEditingGame(game);
    setGameForm({
      name: game.name,
      display_name: game.display_name,
      rules: game.rules || '',
    });
    setShowGameForm(true);
  };

  return (
        <div>
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-semibold text-gray-100">Управление играми</h2>
            <button onClick={() => { setShowGameForm(true); setAdminReaction('typing', '// новая игра?', 2500); }} className="btn btn-primary">
              Добавить игру
            </button>
          </div>

          {/* Game Form Modal */}
          {showGameForm && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-gray-800 rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
                <h2 className="text-xl font-bold mb-4 text-gray-100">
                  {editingGame ? 'Редактировать игру' : 'Создать новую игру'}
                </h2>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-300">
                      Название (уникальный идентификатор)
                    </label>
                    <input
                      type="text"
                      name="gameName"
                      value={gameForm.name}
                      onChange={(e) =>
                        setGameForm({ ...gameForm, name: e.target.value.toLowerCase() })
                      }
                      disabled={!!editingGame}
                      className="input"
                      placeholder="game_name"
                    />
                    <p className="text-xs text-gray-400 mt-1">
                      Только строчные буквы, цифры и подчёркивания
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-300">Отображаемое название</label>
                    <input
                      type="text"
                      name="gameDisplayName"
                      value={gameForm.display_name}
                      onChange={(e) =>
                        setGameForm({ ...gameForm, display_name: e.target.value })
                      }
                      className="input"
                      placeholder="Название игры"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-300">Правила (Markdown)</label>
                    <textarea
                      value={gameForm.rules}
                      onChange={(e) => setGameForm({ ...gameForm, rules: e.target.value })}
                      className="input min-h-[200px] font-mono text-sm"
                      placeholder="# Правила игры&#10;&#10;Напишите правила в формате Markdown..."
                    />
                  </div>

                  {gameError && (
                    <div className="p-2 bg-red-900/30 border border-red-800 rounded text-sm text-red-400">
                      {gameError}
                    </div>
                  )}
                </div>

                <div className="flex justify-end gap-2 mt-6">
                  <button onClick={resetGameForm} className="btn btn-secondary">
                    Отмена
                  </button>
                  <button
                    onClick={handleCreateGame}
                    disabled={isSavingGame}
                    className="btn btn-primary"
                  >
                    {isSavingGame ? 'Сохранение...' : editingGame ? 'Обновить' : 'Создать'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Games List */}
          {games.length === 0 ? (
            <div className="text-center py-8 text-gray-400 bg-gray-800 rounded-lg">
              Игры ещё не созданы.
            </div>
          ) : (
            <div className="space-y-4">
              {games.map((game) => (
                <div key={game.id} className="card flex justify-between items-start">
                  <div>
                    <h3 className="font-semibold text-gray-100">{game.display_name}</h3>
                    <p className="text-sm text-gray-400">
                      <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{game.name}</code>
                    </p>
                    {game.rules && (
                      <p className="text-sm text-gray-300 mt-2 line-clamp-2">
                        {game.rules.substring(0, 150)}...
                      </p>
                    )}
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => startEditGame(game)}
                      className="btn btn-secondary text-sm"
                    >
                      Редактировать
                    </button>
                    {deleteGameId === game.id ? (
                      <div className="flex gap-1">
                        <button
                          onClick={() => handleDeleteGame(game.id)}
                          className="btn btn-danger text-sm"
                        >
                          Подтвердить
                        </button>
                        <button
                          onClick={() => setDeleteGameId(null)}
                          className="btn btn-secondary text-sm"
                        >
                          Отмена
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => setDeleteGameId(game.id)}
                        className="btn btn-danger text-sm"
                      >
                        Удалить
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
  );
}
