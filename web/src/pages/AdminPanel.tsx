import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/client';
import { useAuthStore } from '../store/authStore';
import type { Game, Tournament, TournamentStatus } from '../types';

type AdminTab = 'games' | 'tournaments';

const statusLabels: Record<TournamentStatus, string> = {
  pending: 'Ожидание',
  active: 'Активный',
  completed: 'Завершён',
};

export function AdminPanel() {
  const navigate = useNavigate();
  const { user } = useAuthStore();
  const [activeTab, setActiveTab] = useState<AdminTab>('games');
  const [games, setGames] = useState<Game[]>([]);
  const [tournaments, setTournaments] = useState<Tournament[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  // Game form state
  const [showGameForm, setShowGameForm] = useState(false);
  const [editingGame, setEditingGame] = useState<Game | null>(null);
  const [gameForm, setGameForm] = useState({
    name: '',
    display_name: '',
    rules: '',
  });
  const [isSavingGame, setIsSavingGame] = useState(false);
  const [gameError, setGameError] = useState<string | null>(null);

  // Tournament form state
  const [showTournamentForm, setShowTournamentForm] = useState(false);
  const [tournamentForm, setTournamentForm] = useState({
    name: '',
    description: '',
    game_type: '',
    max_team_size: 3,
    max_participants: '',
    is_permanent: false,
    start_time: '',
    end_time: '',
  });
  const [isSavingTournament, setIsSavingTournament] = useState(false);
  const [tournamentError, setTournamentError] = useState<string | null>(null);

  // Delete confirmation
  const [deleteGameId, setDeleteGameId] = useState<string | null>(null);

  useEffect(() => {
    // Redirect non-admin users
    if (user && user.role !== 'admin') {
      navigate('/');
      return;
    }
    loadData();
  }, [user, navigate]);

  const loadData = async () => {
    setIsLoading(true);
    try {
      const [gamesData, tournamentsData] = await Promise.all([
        api.getGames(),
        api.getTournaments(),
      ]);
      setGames(gamesData || []);
      setTournaments(tournamentsData || []);
    } catch (err) {
      console.error('Failed to load data:', err);
    } finally {
      setIsLoading(false);
    }
  };

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

    try {
      if (editingGame) {
        const updated = await api.updateGame(editingGame.id, {
          display_name: gameForm.display_name,
          rules: gameForm.rules,
        });
        setGames(games.map((g) => (g.id === editingGame.id ? updated : g)));
      } else {
        const newGame = await api.createGame(gameForm);
        setGames([...games, newGame]);
      }
      resetGameForm();
    } catch (err) {
      console.error('Failed to save game:', err);
      setGameError('Не удалось сохранить игру');
    } finally {
      setIsSavingGame(false);
    }
  };

  const handleDeleteGame = async (id: string) => {
    try {
      await api.deleteGame(id);
      setGames(games.filter((g) => g.id !== id));
      setDeleteGameId(null);
    } catch (err) {
      console.error('Failed to delete game:', err);
    }
  };

  const handleCreateTournament = async () => {
    if (!tournamentForm.name.trim()) {
      setTournamentError('Название обязательно');
      return;
    }
    if (!tournamentForm.game_type.trim()) {
      setTournamentError('Тип игры обязателен');
      return;
    }

    setIsSavingTournament(true);
    setTournamentError(null);

    try {
      const payload: Record<string, unknown> = {
        name: tournamentForm.name,
        game_type: tournamentForm.game_type,
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
      setTournaments([...tournaments, newTournament]);
      resetTournamentForm();
    } catch (err) {
      console.error('Failed to create tournament:', err);
      setTournamentError('Не удалось создать турнир');
    } finally {
      setIsSavingTournament(false);
    }
  };

  const resetGameForm = () => {
    setShowGameForm(false);
    setEditingGame(null);
    setGameForm({ name: '', display_name: '', rules: '' });
    setGameError(null);
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
    setTournamentError(null);
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

  if (user?.role !== 'admin') {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">Доступ запрещён. Требуются права администратора.</p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Загрузка...</p>
      </div>
    );
  }

  const tabs: { id: AdminTab; label: string }[] = [
    { id: 'games', label: `Игры (${games.length})` },
    { id: 'tournaments', label: `Турниры (${tournaments.length})` },
  ];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Панель администратора</h1>

      {/* Tabs */}
      <div className="border-b border-gray-200 mb-6">
        <nav className="-mb-px flex gap-4">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`py-2 px-1 border-b-2 font-medium text-sm ${
                activeTab === tab.id
                  ? 'border-primary-500 text-primary-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Games Tab */}
      {activeTab === 'games' && (
        <div>
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-semibold">Управление играми</h2>
            <button onClick={() => setShowGameForm(true)} className="btn btn-primary">
              Добавить игру
            </button>
          </div>

          {/* Game Form Modal */}
          {showGameForm && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-white rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
                <h2 className="text-xl font-bold mb-4">
                  {editingGame ? 'Редактировать игру' : 'Создать новую игру'}
                </h2>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1">
                      Название (уникальный идентификатор)
                    </label>
                    <input
                      type="text"
                      value={gameForm.name}
                      onChange={(e) =>
                        setGameForm({ ...gameForm, name: e.target.value.toLowerCase() })
                      }
                      disabled={!!editingGame}
                      className="input"
                      placeholder="game_name"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      Только строчные буквы, цифры и подчёркивания
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Отображаемое название</label>
                    <input
                      type="text"
                      value={gameForm.display_name}
                      onChange={(e) =>
                        setGameForm({ ...gameForm, display_name: e.target.value })
                      }
                      className="input"
                      placeholder="Название игры"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Правила (Markdown)</label>
                    <textarea
                      value={gameForm.rules}
                      onChange={(e) => setGameForm({ ...gameForm, rules: e.target.value })}
                      className="input min-h-[200px] font-mono text-sm"
                      placeholder="# Правила игры&#10;&#10;Напишите правила в формате Markdown..."
                    />
                  </div>

                  {gameError && (
                    <div className="p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700">
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
            <div className="text-center py-8 text-gray-500">
              Игры ещё не созданы.
            </div>
          ) : (
            <div className="space-y-4">
              {games.map((game) => (
                <div key={game.id} className="card flex justify-between items-start">
                  <div>
                    <h3 className="font-semibold">{game.display_name}</h3>
                    <p className="text-sm text-gray-500">
                      <code>{game.name}</code>
                    </p>
                    {game.rules && (
                      <p className="text-sm text-gray-600 mt-2 line-clamp-2">
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
      )}

      {/* Tournaments Tab */}
      {activeTab === 'tournaments' && (
        <div>
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-semibold">Управление турнирами</h2>
            <button onClick={() => setShowTournamentForm(true)} className="btn btn-primary">
              Создать турнир
            </button>
          </div>

          {/* Tournament Form Modal */}
          {showTournamentForm && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-white rounded-lg p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto">
                <h2 className="text-xl font-bold mb-4">Создать турнир</h2>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1">Название *</label>
                    <input
                      type="text"
                      value={tournamentForm.name}
                      onChange={(e) =>
                        setTournamentForm({ ...tournamentForm, name: e.target.value })
                      }
                      className="input"
                      placeholder="Название турнира"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Тип игры *</label>
                    <select
                      value={tournamentForm.game_type}
                      onChange={(e) =>
                        setTournamentForm({ ...tournamentForm, game_type: e.target.value })
                      }
                      className="input"
                    >
                      <option value="">Выберите игру</option>
                      {games.map((game) => (
                        <option key={game.id} value={game.name}>
                          {game.display_name}
                        </option>
                      ))}
                    </select>
                    <p className="text-xs text-gray-500 mt-1">
                      Сначала создайте игру во вкладке "Игры"
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Описание</label>
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
                      <label className="block text-sm font-medium mb-1">Макс. размер команды</label>
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
                      <label className="block text-sm font-medium mb-1">Макс. участников</label>
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
                      <label className="block text-sm font-medium mb-1">Дата начала</label>
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
                      <label className="block text-sm font-medium mb-1">Дата окончания</label>
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
                    <label htmlFor="is_permanent" className="text-sm">
                      Постоянный турнир (всегда принимает новых участников)
                    </label>
                  </div>

                  {tournamentError && (
                    <div className="p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700">
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

          {/* Tournaments List */}
          {tournaments.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              Турниры ещё не созданы.
            </div>
          ) : (
            <div className="space-y-4">
              {tournaments.map((tournament) => (
                <div key={tournament.id} className="card">
                  <div className="flex justify-between items-start">
                    <div>
                      <h3 className="font-semibold">{tournament.name}</h3>
                      <p className="text-sm text-gray-500">
                        Код: <code>{tournament.code}</code>
                      </p>
                      {tournament.description && (
                        <p className="text-sm text-gray-600 mt-1 line-clamp-2">
                          {tournament.description}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <span
                        className={`px-2 py-1 rounded text-xs font-medium ${
                          tournament.status === 'pending'
                            ? 'bg-yellow-100 text-yellow-800'
                            : tournament.status === 'active'
                            ? 'bg-green-100 text-green-800'
                            : 'bg-gray-100 text-gray-800'
                        }`}
                      >
                        {statusLabels[tournament.status]}
                      </span>
                      {tournament.is_permanent && (
                        <span className="bg-blue-100 text-blue-800 px-2 py-1 rounded text-xs font-medium">
                          Постоянный
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="mt-3 flex gap-2">
                    <a
                      href={`/tournaments/${tournament.id}`}
                      className="btn btn-secondary text-sm"
                    >
                      Просмотр
                    </a>
                    {tournament.status === 'pending' && (
                      <button
                        onClick={async () => {
                          await api.startTournament(tournament.id);
                          loadData();
                        }}
                        className="btn btn-primary text-sm"
                      >
                        Запустить
                      </button>
                    )}
                    {tournament.status === 'active' && (
                      <button
                        onClick={async () => {
                          await api.completeTournament(tournament.id);
                          loadData();
                        }}
                        className="btn btn-secondary text-sm"
                      >
                        Завершить
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
