import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/client';
import { useAuthStore } from '../store/authStore';
import type { Game, Tournament, TournamentStatus, LeaderboardEntry } from '../types';

type AdminTab = 'games' | 'tournaments' | 'programs';

// Game-specific icons configuration for programs view
const gameIcons: Record<string, string> = {
  prisoners_dilemma: '🤝',
  tug_of_war: '🪢',
  good_deal: '💰',
  balance_of_universe: '⚖️',
};
const getGameIcon = (gameName: string) => gameIcons[gameName] || '🎮';

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
  const [selectedGameIds, setSelectedGameIds] = useState<string[]>([]);
  const [isSavingTournament, setIsSavingTournament] = useState(false);
  const [tournamentError, setTournamentError] = useState<string | null>(null);

  // Delete confirmation
  const [deleteGameId, setDeleteGameId] = useState<string | null>(null);
  const [deleteTournamentId, setDeleteTournamentId] = useState<string | null>(null);

  // Action errors
  const [actionError, setActionError] = useState<string | null>(null);

  // Programs tab state
  const [selectedTournamentId, setSelectedTournamentId] = useState<string | null>(null);
  const [tournamentGames, setTournamentGames] = useState<Game[]>([]);
  const [programsData, setProgramsData] = useState<Record<string, LeaderboardEntry[]>>({});
  const [isLoadingPrograms, setIsLoadingPrograms] = useState(false);

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

  const handleDeleteTournament = async (id: string) => {
    try {
      await api.deleteTournament(id);
      setTournaments(tournaments.filter((t) => t.id !== id));
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
    try {
      await api.startTournament(id);
      loadData();
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

  const startEditGame = (game: Game) => {
    setEditingGame(game);
    setGameForm({
      name: game.name,
      display_name: game.display_name,
      rules: game.rules || '',
    });
    setShowGameForm(true);
  };

  // Load programs for selected tournament
  const loadTournamentPrograms = async (tournamentId: string) => {
    setIsLoadingPrograms(true);
    setProgramsData({});

    try {
      // Get games for this tournament
      const gamesData = await api.getTournamentGames(tournamentId);
      setTournamentGames(gamesData);

      // Load leaderboard for each game (contains program info)
      const programsByGame: Record<string, LeaderboardEntry[]> = {};

      // First, try to get game-specific leaderboards
      for (const game of gamesData) {
        try {
          const leaderboard = await api.getGameLeaderboard(tournamentId, game.id);
          if (leaderboard && leaderboard.length > 0) {
            programsByGame[game.id] = leaderboard;
          }
        } catch {
          console.error(`Failed to load leaderboard for game ${game.id}`);
        }
      }

      // If no game-specific data, fall back to tournament-level leaderboard
      if (Object.keys(programsByGame).length === 0) {
        try {
          const tournamentLeaderboard = await api.getLeaderboard(tournamentId);
          if (tournamentLeaderboard && tournamentLeaderboard.length > 0) {
            // Put all programs under "all" key or first game
            const key = gamesData.length > 0 ? gamesData[0].id : 'all';
            programsByGame[key] = tournamentLeaderboard;
          }
        } catch {
          console.error('Failed to load tournament leaderboard');
        }
      }

      setProgramsData(programsByGame);
    } catch (err) {
      console.error('Failed to load tournament programs:', err);
    } finally {
      setIsLoadingPrograms(false);
    }
  };

  const handleTournamentSelect = (tournamentId: string) => {
    setSelectedTournamentId(tournamentId);
    loadTournamentPrograms(tournamentId);
  };

  if (user?.role !== 'admin') {
    return (
      <div className="text-center py-12">
        <p className="text-red-500 dark:text-red-400">Доступ запрещён. Требуются права администратора.</p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500 dark:text-gray-400">Загрузка...</p>
      </div>
    );
  }

  const tabs: { id: AdminTab; label: string }[] = [
    { id: 'games', label: `Игры (${games.length})` },
    { id: 'tournaments', label: `Турниры (${tournaments.length})` },
    { id: 'programs', label: 'Программы' },
  ];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6 text-gray-900 dark:text-gray-100">Панель администратора</h1>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700 mb-6">
        <nav className="-mb-px flex gap-4">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`py-2 px-1 border-b-2 font-medium text-sm ${
                activeTab === tab.id
                  ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                  : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
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
            <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Управление играми</h2>
            <button onClick={() => setShowGameForm(true)} className="btn btn-primary">
              Добавить игру
            </button>
          </div>

          {/* Game Form Modal */}
          {showGameForm && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
                <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-gray-100">
                  {editingGame ? 'Редактировать игру' : 'Создать новую игру'}
                </h2>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">
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
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                      Только строчные буквы, цифры и подчёркивания
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Отображаемое название</label>
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
                    <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Правила (Markdown)</label>
                    <textarea
                      value={gameForm.rules}
                      onChange={(e) => setGameForm({ ...gameForm, rules: e.target.value })}
                      className="input min-h-[200px] font-mono text-sm"
                      placeholder="# Правила игры&#10;&#10;Напишите правила в формате Markdown..."
                    />
                  </div>

                  {gameError && (
                    <div className="p-2 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded text-sm text-red-700 dark:text-red-400">
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
            <div className="text-center py-8 text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 rounded-lg">
              Игры ещё не созданы.
            </div>
          ) : (
            <div className="space-y-4">
              {games.map((game) => (
                <div key={game.id} className="card flex justify-between items-start">
                  <div>
                    <h3 className="font-semibold text-gray-900 dark:text-gray-100">{game.display_name}</h3>
                    <p className="text-sm text-gray-500 dark:text-gray-400">
                      <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{game.name}</code>
                    </p>
                    {game.rules && (
                      <p className="text-sm text-gray-600 dark:text-gray-300 mt-2 line-clamp-2">
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
            <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Управление турнирами</h2>
            <button onClick={() => setShowTournamentForm(true)} className="btn btn-primary">
              Создать турнир
            </button>
          </div>

          {/* Tournament Form Modal */}
          {showTournamentForm && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto">
                <h2 className="text-xl font-bold mb-4 text-gray-900 dark:text-gray-100">Создать турнир</h2>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Название *</label>
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
                    <label className="block text-sm font-medium mb-2 text-gray-700 dark:text-gray-300">Игры турнира *</label>
                    {games.length === 0 ? (
                      <p className="text-sm text-gray-500 dark:text-gray-400">
                        Сначала создайте игры во вкладке "Игры"
                      </p>
                    ) : (
                      <div className="space-y-2 max-h-48 overflow-y-auto border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-700">
                        {games.map((game) => (
                          <label
                            key={game.id}
                            className="flex items-center gap-3 p-2 hover:bg-gray-50 dark:hover:bg-gray-600 rounded cursor-pointer"
                          >
                            <input
                              type="checkbox"
                              checked={selectedGameIds.includes(game.id)}
                              onChange={() => toggleGameSelection(game.id)}
                              className="w-4 h-4 text-primary-600 rounded"
                            />
                            <div>
                              <span className="font-medium text-gray-900 dark:text-gray-100">{game.display_name}</span>
                              <span className="text-xs text-gray-500 dark:text-gray-400 ml-2">({game.name})</span>
                            </div>
                          </label>
                        ))}
                      </div>
                    )}
                    {selectedGameIds.length > 0 && (
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        Выбрано игр: {selectedGameIds.length}
                      </p>
                    )}
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Описание</label>
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
                      <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Макс. размер команды</label>
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
                      <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Макс. участников</label>
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
                      <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Дата начала</label>
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
                      <label className="block text-sm font-medium mb-1 text-gray-700 dark:text-gray-300">Дата окончания</label>
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
                    <label htmlFor="is_permanent" className="text-sm text-gray-700 dark:text-gray-300">
                      Постоянный турнир (всегда принимает новых участников)
                    </label>
                  </div>

                  {tournamentError && (
                    <div className="p-2 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded text-sm text-red-700 dark:text-red-400">
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
            <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-700 dark:text-red-400">
              {actionError}
              <button
                onClick={() => setActionError(null)}
                className="ml-2 text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
              >
                ✕
              </button>
            </div>
          )}

          {/* Tournaments List */}
          {tournaments.length === 0 ? (
            <div className="text-center py-8 text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 rounded-lg">
              Турниры ещё не созданы.
            </div>
          ) : (
            <div className="space-y-4">
              {tournaments.map((tournament) => (
                <div key={tournament.id} className="card">
                  <div className="flex justify-between items-start">
                    <div>
                      <h3 className="font-semibold text-gray-900 dark:text-gray-100">{tournament.name}</h3>
                      <p className="text-sm text-gray-500 dark:text-gray-400">
                        Код: <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{tournament.code}</code>
                      </p>
                      {tournament.description && (
                        <p className="text-sm text-gray-600 dark:text-gray-300 mt-1 line-clamp-2">
                          {tournament.description}
                        </p>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <span
                        className={`px-2 py-1 rounded text-xs font-medium ${
                          tournament.status === 'pending'
                            ? 'bg-yellow-100 dark:bg-yellow-900/50 text-yellow-800 dark:text-yellow-300'
                            : tournament.status === 'active'
                            ? 'bg-green-100 dark:bg-green-900/50 text-green-800 dark:text-green-300'
                            : 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-300'
                        }`}
                      >
                        {statusLabels[tournament.status]}
                      </span>
                      {tournament.is_permanent && (
                        <span className="bg-blue-100 dark:bg-blue-900/50 text-blue-800 dark:text-blue-300 px-2 py-1 rounded text-xs font-medium">
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
      )}

      {/* Programs Tab */}
      {activeTab === 'programs' && (
        <div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100 mb-4">Просмотр загруженных программ</h2>

          {/* Tournament selector */}
          <div className="mb-6">
            <label className="block text-sm font-medium mb-2 text-gray-700 dark:text-gray-300">
              Выберите турнир
            </label>
            <select
              value={selectedTournamentId || ''}
              onChange={(e) => e.target.value && handleTournamentSelect(e.target.value)}
              className="input max-w-md"
            >
              <option value="">-- Выберите турнир --</option>
              {tournaments.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name} ({statusLabels[t.status]})
                </option>
              ))}
            </select>
          </div>

          {/* Loading state */}
          {isLoadingPrograms && (
            <div className="text-center py-8 text-gray-500 dark:text-gray-400">
              Загрузка программ...
            </div>
          )}

          {/* No tournament selected */}
          {!selectedTournamentId && !isLoadingPrograms && (
            <div className="text-center py-8 text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 rounded-lg">
              Выберите турнир для просмотра загруженных программ
            </div>
          )}

          {/* Programs data */}
          {selectedTournamentId && !isLoadingPrograms && (
            <div className="space-y-6">
              {tournamentGames.length === 0 ? (
                <div className="text-center py-8 text-gray-500 dark:text-gray-400 bg-gray-50 dark:bg-gray-800 rounded-lg">
                  В этом турнире нет игр
                </div>
              ) : (
                tournamentGames.map((game) => {
                  const programs = programsData[game.id] || [];
                  const totalPrograms = programs.length;

                  return (
                    <div key={game.id} className="card">
                      <div className="flex items-center gap-3 mb-4">
                        <span className="text-2xl">{getGameIcon(game.name)}</span>
                        <div>
                          <h3 className="font-semibold text-gray-900 dark:text-gray-100">
                            {game.display_name}
                          </h3>
                          <p className="text-sm text-gray-500 dark:text-gray-400">
                            {totalPrograms} {totalPrograms === 1 ? 'программа' : totalPrograms < 5 ? 'программы' : 'программ'}
                          </p>
                        </div>
                      </div>

                      {programs.length === 0 ? (
                        <p className="text-sm text-gray-500 dark:text-gray-400">
                          Программы ещё не загружены
                        </p>
                      ) : (
                        <div className="overflow-x-auto">
                          <table className="w-full">
                            <thead>
                              <tr className="text-left text-sm text-gray-500 dark:text-gray-400 border-b dark:border-gray-700">
                                <th className="pb-2 pr-4">#</th>
                                <th className="pb-2 pr-4">Программа</th>
                                <th className="pb-2 pr-4">Команда</th>
                                <th className="pb-2 pr-4 text-center">Рейтинг</th>
                                <th className="pb-2 pr-4 text-center">W</th>
                                <th className="pb-2 pr-4 text-center">L</th>
                                <th className="pb-2 pr-4 text-center">D</th>
                                <th className="pb-2 text-center">Игр</th>
                              </tr>
                            </thead>
                            <tbody>
                              {programs.map((entry) => (
                                <tr key={entry.program_id} className="border-b border-gray-100 dark:border-gray-800">
                                  <td className="py-2 pr-4 font-medium text-gray-600 dark:text-gray-400">{entry.rank}</td>
                                  <td className="py-2 pr-4">
                                    <div className="font-medium text-gray-900 dark:text-gray-100">
                                      {entry.program_name}
                                    </div>
                                    <code className="text-xs text-gray-500 dark:text-gray-500 font-mono">
                                      {entry.program_id.substring(0, 8)}...
                                    </code>
                                  </td>
                                  <td className="py-2 pr-4 text-gray-600 dark:text-gray-300">
                                    {entry.team_name || '-'}
                                  </td>
                                  <td className="py-2 pr-4 text-center font-bold text-gray-900 dark:text-gray-100">
                                    {entry.rating}
                                  </td>
                                  <td className="py-2 pr-4 text-center text-green-600 dark:text-green-400">
                                    {entry.wins}
                                  </td>
                                  <td className="py-2 pr-4 text-center text-red-600 dark:text-red-400">
                                    {entry.losses}
                                  </td>
                                  <td className="py-2 pr-4 text-center text-gray-500 dark:text-gray-400">
                                    {entry.draws}
                                  </td>
                                  <td className="py-2 text-center text-gray-600 dark:text-gray-300">
                                    {entry.total_games}
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        </div>
                      )}
                    </div>
                  );
                })
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
