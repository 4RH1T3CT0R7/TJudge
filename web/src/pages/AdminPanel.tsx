import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/client';
import { useAuthStore } from '../store/authStore';
import type { Game, Tournament, TournamentStatus, LeaderboardEntry, QueueStats, MatchStatistics, Program, SystemMetrics } from '../types';

type AdminTab = 'games' | 'tournaments' | 'programs' | 'system';

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

// Helper function to format bytes to human readable format
const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
};

// Helper function to format uptime to human readable format
const formatUptime = (seconds: number): string => {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  const parts = [];
  if (days > 0) parts.push(`${days}д`);
  if (hours > 0) parts.push(`${hours}ч`);
  if (minutes > 0 || parts.length === 0) parts.push(`${minutes}м`);

  return parts.join(' ');
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

  // Tournament games management state
  const [managingTournamentId, setManagingTournamentId] = useState<string | null>(null);
  const [managingTournamentGames, setManagingTournamentGames] = useState<Game[]>([]);
  const [isLoadingTournamentGames, setIsLoadingTournamentGames] = useState(false);
  const [runningGameMatches, setRunningGameMatches] = useState<string | null>(null);

  // Programs tab state
  const [selectedTournamentId, setSelectedTournamentId] = useState<string | null>(null);
  const [tournamentGames, setTournamentGames] = useState<Game[]>([]);
  const [programsData, setProgramsData] = useState<Record<string, LeaderboardEntry[]>>({});
  const [programDetails, setProgramDetails] = useState<Record<string, Program[]>>({});
  const [isLoadingPrograms, setIsLoadingPrograms] = useState(false);

  // System tab state
  const [queueStats, setQueueStats] = useState<QueueStats | null>(null);
  const [matchStats, setMatchStats] = useState<MatchStatistics | null>(null);
  const [systemMetrics, setSystemMetrics] = useState<SystemMetrics | null>(null);
  const [isLoadingSystem, setIsLoadingSystem] = useState(false);
  const [systemError, setSystemError] = useState<string | null>(null);
  const [isClearing, setIsClearing] = useState(false);
  const [isPurging, setIsPurging] = useState(false);

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

  // System data loading
  const loadSystemData = useCallback(async () => {
    setIsLoadingSystem(true);
    setSystemError(null);
    try {
      const [queueData, matchData, metricsData] = await Promise.all([
        api.getQueueStats(),
        api.getMatchStatistics(),
        api.getSystemMetrics(),
      ]);
      setQueueStats(queueData);
      setMatchStats(matchData);
      setSystemMetrics(metricsData);
    } catch (err) {
      console.error('Failed to load system data:', err);
      setSystemError('Не удалось загрузить данные системы');
    } finally {
      setIsLoadingSystem(false);
    }
  }, []);

  // Auto-refresh system data when on system tab
  useEffect(() => {
    if (activeTab === 'system') {
      loadSystemData();
      const interval = setInterval(loadSystemData, 5000); // Refresh every 5 seconds
      return () => clearInterval(interval);
    }
  }, [activeTab, loadSystemData]);

  const handleClearQueue = async () => {
    if (!confirm('Вы уверены, что хотите очистить очередь? Все ожидающие матчи будут удалены.')) {
      return;
    }
    setIsClearing(true);
    setSystemError(null);
    try {
      await api.clearQueue();
      loadSystemData();
    } catch (err) {
      console.error('Failed to clear queue:', err);
      setSystemError('Не удалось очистить очередь');
    } finally {
      setIsClearing(false);
    }
  };

  const handlePurgeInvalidMatches = async () => {
    setIsPurging(true);
    setSystemError(null);
    try {
      const result = await api.purgeInvalidMatches();
      alert(`Удалено ${result.purged_count} невалидных матчей из очереди`);
      loadSystemData();
    } catch (err) {
      console.error('Failed to purge invalid matches:', err);
      setSystemError('Не удалось очистить невалидные матчи');
    } finally {
      setIsPurging(false);
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
    setProgramDetails({});

    try {
      // Get games for this tournament
      const gamesData = await api.getTournamentGames(tournamentId);
      setTournamentGames(gamesData);

      // Load leaderboard and program details for each game
      const programsByGame: Record<string, LeaderboardEntry[]> = {};
      const detailsByGame: Record<string, Program[]> = {};

      // First, try to get game-specific leaderboards and program details
      for (const game of gamesData) {
        try {
          const leaderboard = await api.getGameLeaderboard(tournamentId, game.id);
          if (leaderboard && leaderboard.length > 0) {
            programsByGame[game.id] = leaderboard;
          }
        } catch {
          console.error(`Failed to load leaderboard for game ${game.id}`);
        }

        // Load full program details (includes error_message)
        try {
          const programs = await api.getGamePrograms(tournamentId, game.id);
          if (programs && programs.length > 0) {
            detailsByGame[game.id] = programs;
          }
        } catch {
          console.error(`Failed to load programs for game ${game.id}`);
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
      setProgramDetails(detailsByGame);
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

  // Download program file
  const handleDownloadProgram = async (programId: string, programName: string) => {
    try {
      const blob = await api.downloadProgram(programId);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${programName}.py`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (err) {
      console.error('Failed to download program:', err);
      setActionError('Не удалось скачать программу');
    }
  };

  // Open tournament games management modal
  const openTournamentGamesManagement = async (tournamentId: string) => {
    setManagingTournamentId(tournamentId);
    setIsLoadingTournamentGames(true);
    try {
      const gamesData = await api.getTournamentGames(tournamentId);
      setManagingTournamentGames(gamesData || []);
    } catch (err) {
      console.error('Failed to load tournament games:', err);
      setActionError('Не удалось загрузить игры турнира');
    } finally {
      setIsLoadingTournamentGames(false);
    }
  };

  // Close tournament games management modal
  const closeTournamentGamesManagement = () => {
    setManagingTournamentId(null);
    setManagingTournamentGames([]);
    setRunningGameMatches(null);
  };

  // Run matches for a specific game
  const handleRunGameMatches = async (gameType: string, gameName: string) => {
    if (!managingTournamentId) return;

    setRunningGameMatches(gameType);
    setActionError(null);

    try {
      const result = await api.runGameMatches(managingTournamentId, gameType);
      setActionError(null);
      // Show success message
      alert(`Запущено ${result.enqueued} матчей для "${gameName}"`);
    } catch (err: unknown) {
      console.error('Failed to run game matches:', err);
      const axiosErr = err as { response?: { data?: { message?: string } } };
      setActionError(axiosErr.response?.data?.message || 'Не удалось запустить матчи');
    } finally {
      setRunningGameMatches(null);
    }
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
    { id: 'system', label: 'Система' },
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
                      <div className="space-y-3">
                        {/* Available games */}
                        <div className="space-y-2 max-h-32 overflow-y-auto border border-gray-200 dark:border-gray-600 rounded-lg p-3 bg-white dark:bg-gray-700">
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

                        {/* Selected games with order controls */}
                        {selectedGameIds.length > 0 && (
                          <div>
                            <p className="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                              Порядок игр (раунды будут запускаться в этом порядке):
                            </p>
                            <div className="space-y-2 border border-primary-200 dark:border-primary-800 rounded-lg p-3 bg-primary-50 dark:bg-primary-900/20">
                              {selectedGameIds.map((gameId, index) => {
                                const game = games.find(g => g.id === gameId);
                                if (!game) return null;
                                return (
                                  <div
                                    key={gameId}
                                    className="flex items-center justify-between p-2 bg-white dark:bg-gray-800 rounded border border-gray-200 dark:border-gray-700"
                                  >
                                    <div className="flex items-center gap-2">
                                      <span className="text-sm font-bold text-primary-600 dark:text-primary-400 w-6">
                                        {index + 1}.
                                      </span>
                                      <span className="text-lg">{getGameIcon(game.name)}</span>
                                      <span className="font-medium text-gray-900 dark:text-gray-100">
                                        {game.display_name}
                                      </span>
                                    </div>
                                    <div className="flex items-center gap-1">
                                      <button
                                        type="button"
                                        onClick={() => moveGameUp(index)}
                                        disabled={index === 0}
                                        className="p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 disabled:opacity-30"
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
                                        className="p-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 disabled:opacity-30"
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

          {/* Tournament Games Management Modal */}
          {managingTournamentId && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-white dark:bg-gray-800 rounded-lg p-6 w-full max-w-lg max-h-[90vh] overflow-y-auto">
                <div className="flex justify-between items-center mb-4">
                  <h2 className="text-xl font-bold text-gray-900 dark:text-gray-100">
                    Запустить раунд по игре
                  </h2>
                  <button
                    onClick={closeTournamentGamesManagement}
                    className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300"
                  >
                    ✕
                  </button>
                </div>

                <p className="text-sm text-gray-600 dark:text-gray-400 mb-4">
                  Выберите игру для запуска раунда матчей. Раунд создаст матчи для всех участников и добавит их в очередь.
                </p>

                {isLoadingTournamentGames ? (
                  <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                    Загрузка игр...
                  </div>
                ) : managingTournamentGames.length === 0 ? (
                  <div className="text-center py-8 text-gray-500 dark:text-gray-400">
                    В этом турнире нет игр
                  </div>
                ) : (
                  <div className="space-y-3">
                    {managingTournamentGames.map((game) => (
                      <div
                        key={game.id}
                        className="flex items-center justify-between p-3 border border-gray-200 dark:border-gray-700 rounded-lg"
                      >
                        <div className="flex items-center gap-3">
                          <span className="text-2xl">{getGameIcon(game.name)}</span>
                          <div>
                            <p className="font-medium text-gray-900 dark:text-gray-100">
                              {game.display_name}
                            </p>
                            <p className="text-xs text-gray-500 dark:text-gray-400">
                              {game.name}
                            </p>
                          </div>
                        </div>
                        <button
                          onClick={() => handleRunGameMatches(game.name, game.display_name)}
                          disabled={runningGameMatches === game.name}
                          className="btn btn-primary text-sm disabled:opacity-50"
                        >
                          {runningGameMatches === game.name ? 'Запуск...' : 'Запустить раунд'}
                        </button>
                      </div>
                    ))}
                  </div>
                )}

                <div className="flex justify-end mt-6">
                  <button onClick={closeTournamentGamesManagement} className="btn btn-secondary">
                    Закрыть
                  </button>
                </div>
              </div>
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
                      <>
                        <button
                          onClick={() => openTournamentGamesManagement(tournament.id)}
                          className="btn btn-primary text-sm"
                        >
                          Запустить раунд
                        </button>
                        <button
                          onClick={async () => {
                            await api.completeTournament(tournament.id);
                            loadData();
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
                  const details = programDetails[game.id] || [];
                  const totalPrograms = programs.length || details.length;

                  // Create a lookup map for program errors
                  const errorLookup = new Map<string, string>();
                  details.forEach(p => {
                    if (p.error_message) {
                      errorLookup.set(p.id, p.error_message);
                    }
                  });

                  // Count programs with errors
                  const programsWithErrors = details.filter(p => p.error_message).length;

                  return (
                    <div key={game.id} className="card">
                      <div className="flex items-center justify-between mb-4">
                        <div className="flex items-center gap-3">
                          <span className="text-2xl">{getGameIcon(game.name)}</span>
                          <div>
                            <h3 className="font-semibold text-gray-900 dark:text-gray-100">
                              {game.display_name}
                            </h3>
                            <div className="flex items-center gap-2">
                              <p className="text-sm text-gray-500 dark:text-gray-400">
                                {totalPrograms} {totalPrograms === 1 ? 'программа' : totalPrograms < 5 ? 'программы' : 'программ'}
                              </p>
                              {programsWithErrors > 0 && (
                                <span className="px-2 py-0.5 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 text-xs rounded-full">
                                  {programsWithErrors} с ошибкой
                                </span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>

                      {programs.length === 0 && details.length === 0 ? (
                        <p className="text-sm text-gray-500 dark:text-gray-400">
                          Программы ещё не загружены
                        </p>
                      ) : programs.length > 0 ? (
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
                                <th className="pb-2 pr-4 text-center">Игр</th>
                                <th className="pb-2 pr-4">Статус</th>
                                <th className="pb-2">Действия</th>
                              </tr>
                            </thead>
                            <tbody>
                              {programs.map((entry) => {
                                const error = errorLookup.get(entry.program_id);
                                return (
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
                                    <td className="py-2 pr-4 text-center text-gray-600 dark:text-gray-300">
                                      {entry.total_games}
                                    </td>
                                    <td className="py-2 pr-4">
                                      {error ? (
                                        <div className="group relative">
                                          <span className="px-2 py-1 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 text-xs rounded cursor-help">
                                            Ошибка
                                          </span>
                                          <div className="absolute z-10 hidden group-hover:block w-80 p-2 bg-gray-900 text-white text-xs rounded shadow-lg -left-32 top-full mt-1">
                                            <pre className="whitespace-pre-wrap break-words font-mono">{error}</pre>
                                          </div>
                                        </div>
                                      ) : (
                                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 text-xs rounded">
                                          OK
                                        </span>
                                      )}
                                    </td>
                                    <td className="py-2">
                                      <button
                                        onClick={() => handleDownloadProgram(entry.program_id, entry.program_name)}
                                        className="text-primary-600 dark:text-primary-400 hover:text-primary-800 dark:hover:text-primary-300 text-sm"
                                        title="Скачать программу"
                                      >
                                        ⬇️ Скачать
                                      </button>
                                    </td>
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        </div>
                      ) : (
                        // Show details only if no leaderboard but have program details
                        <div className="overflow-x-auto">
                          <table className="w-full">
                            <thead>
                              <tr className="text-left text-sm text-gray-500 dark:text-gray-400 border-b dark:border-gray-700">
                                <th className="pb-2 pr-4">Программа</th>
                                <th className="pb-2 pr-4">Версия</th>
                                <th className="pb-2 pr-4">Язык</th>
                                <th className="pb-2 pr-4">Статус</th>
                                <th className="pb-2">Действия</th>
                              </tr>
                            </thead>
                            <tbody>
                              {details.map((prog) => (
                                <tr key={prog.id} className="border-b border-gray-100 dark:border-gray-800">
                                  <td className="py-2 pr-4">
                                    <div className="font-medium text-gray-900 dark:text-gray-100">
                                      {prog.name}
                                    </div>
                                    <code className="text-xs text-gray-500 dark:text-gray-500 font-mono">
                                      {prog.id.substring(0, 8)}...
                                    </code>
                                  </td>
                                  <td className="py-2 pr-4 text-gray-600 dark:text-gray-300">
                                    v{prog.version}
                                  </td>
                                  <td className="py-2 pr-4 text-gray-600 dark:text-gray-300">
                                    {prog.language}
                                  </td>
                                  <td className="py-2 pr-4">
                                    {prog.error_message ? (
                                      <div className="group relative">
                                        <span className="px-2 py-1 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 text-xs rounded cursor-help">
                                          Ошибка
                                        </span>
                                        <div className="absolute z-10 hidden group-hover:block w-80 p-2 bg-gray-900 text-white text-xs rounded shadow-lg -left-32 top-full mt-1">
                                          <pre className="whitespace-pre-wrap break-words font-mono">{prog.error_message}</pre>
                                        </div>
                                      </div>
                                    ) : (
                                      <span className="px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 text-xs rounded">
                                        OK
                                      </span>
                                    )}
                                  </td>
                                  <td className="py-2">
                                    <button
                                      onClick={() => handleDownloadProgram(prog.id, prog.name)}
                                      className="text-primary-600 dark:text-primary-400 hover:text-primary-800 dark:hover:text-primary-300 text-sm"
                                      title="Скачать программу"
                                    >
                                      ⬇️ Скачать
                                    </button>
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

      {/* System Tab */}
      {activeTab === 'system' && (
        <div>
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Состояние системы</h2>
            <button
              onClick={loadSystemData}
              disabled={isLoadingSystem}
              className="btn btn-secondary text-sm"
            >
              {isLoadingSystem ? 'Обновление...' : 'Обновить'}
            </button>
          </div>

          {systemError && (
            <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded text-sm text-red-700 dark:text-red-400">
              {systemError}
            </div>
          )}

          {isLoadingSystem && !queueStats && !matchStats ? (
            <div className="text-center py-8 text-gray-500 dark:text-gray-400">
              Загрузка данных системы...
            </div>
          ) : (
            <div className="grid gap-6 md:grid-cols-2">
              {/* Queue Stats Card */}
              <div className="card">
                <h3 className="text-md font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">📊</span>
                  Очередь матчей
                </h3>
                {queueStats ? (
                  <div className="space-y-3">
                    <div className="flex justify-between items-center py-2 border-b border-gray-100 dark:border-gray-700">
                      <span className="text-gray-600 dark:text-gray-400">Всего в очереди</span>
                      <span className="text-2xl font-bold text-gray-900 dark:text-gray-100">{queueStats.total}</span>
                    </div>
                    <div className="grid grid-cols-3 gap-3 pt-2">
                      <div className="text-center">
                        <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Высокий</div>
                        <div className="text-lg font-semibold text-red-600 dark:text-red-400">{queueStats.high}</div>
                      </div>
                      <div className="text-center">
                        <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Средний</div>
                        <div className="text-lg font-semibold text-yellow-600 dark:text-yellow-400">{queueStats.medium}</div>
                      </div>
                      <div className="text-center">
                        <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Низкий</div>
                        <div className="text-lg font-semibold text-blue-600 dark:text-blue-400">{queueStats.low}</div>
                      </div>
                    </div>
                  </div>
                ) : (
                  <p className="text-gray-500 dark:text-gray-400">Нет данных</p>
                )}
              </div>

              {/* Match Stats Card */}
              <div className="card">
                <h3 className="text-md font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">🎮</span>
                  Статистика матчей
                </h3>
                {matchStats ? (
                  <div className="space-y-3">
                    <div className="flex justify-between items-center py-2 border-b border-gray-100 dark:border-gray-700">
                      <span className="text-gray-600 dark:text-gray-400">Всего матчей</span>
                      <span className="text-2xl font-bold text-gray-900 dark:text-gray-100">{matchStats.total}</span>
                    </div>
                    <div className="grid grid-cols-2 gap-3 pt-2">
                      <div className="flex justify-between items-center">
                        <span className="text-gray-600 dark:text-gray-400">Ожидают</span>
                        <span className="px-2 py-1 bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400 rounded font-medium">{matchStats.pending}</span>
                      </div>
                      <div className="flex justify-between items-center">
                        <span className="text-gray-600 dark:text-gray-400">Выполняются</span>
                        <span className="px-2 py-1 bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 rounded font-medium">{matchStats.running}</span>
                      </div>
                      <div className="flex justify-between items-center">
                        <span className="text-gray-600 dark:text-gray-400">Завершены</span>
                        <span className="px-2 py-1 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 rounded font-medium">{matchStats.completed}</span>
                      </div>
                      <div className="flex justify-between items-center">
                        <span className="text-gray-600 dark:text-gray-400">С ошибкой</span>
                        <span className="px-2 py-1 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400 rounded font-medium">{matchStats.failed}</span>
                      </div>
                    </div>
                  </div>
                ) : (
                  <p className="text-gray-500 dark:text-gray-400">Нет данных</p>
                )}
              </div>

              {/* System Metrics Card */}
              <div className="card md:col-span-2">
                <h3 className="text-md font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">💻</span>
                  Нагрузка сервера
                </h3>
                {systemMetrics ? (
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                    {/* CPU */}
                    <div className="space-y-3">
                      <div className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                        <span>🔧</span> CPU
                      </div>
                      <div className="relative pt-1">
                        <div className="flex mb-2 items-center justify-between">
                          <span className="text-xs font-semibold inline-block text-gray-600 dark:text-gray-400">
                            {systemMetrics.cpu.usage_percent.toFixed(1)}%
                          </span>
                          <span className="text-xs text-gray-500 dark:text-gray-400">
                            {systemMetrics.cpu.cores} ядер
                          </span>
                        </div>
                        <div className="overflow-hidden h-2 text-xs flex rounded bg-gray-200 dark:bg-gray-700">
                          <div
                            style={{ width: `${Math.min(systemMetrics.cpu.usage_percent, 100)}%` }}
                            className={`shadow-none flex flex-col text-center whitespace-nowrap text-white justify-center transition-all duration-300 ${
                              systemMetrics.cpu.usage_percent > 80
                                ? 'bg-red-500'
                                : systemMetrics.cpu.usage_percent > 50
                                ? 'bg-yellow-500'
                                : 'bg-green-500'
                            }`}
                          />
                        </div>
                      </div>
                      {systemMetrics.cpu.model_name && (
                        <p className="text-xs text-gray-500 dark:text-gray-400 truncate" title={systemMetrics.cpu.model_name}>
                          {systemMetrics.cpu.model_name}
                        </p>
                      )}
                    </div>

                    {/* Memory */}
                    <div className="space-y-3">
                      <div className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                        <span>🧠</span> Память
                      </div>
                      <div className="relative pt-1">
                        <div className="flex mb-2 items-center justify-between">
                          <span className="text-xs font-semibold inline-block text-gray-600 dark:text-gray-400">
                            {systemMetrics.memory.used_percent.toFixed(1)}%
                          </span>
                          <span className="text-xs text-gray-500 dark:text-gray-400">
                            {formatBytes(systemMetrics.memory.used)} / {formatBytes(systemMetrics.memory.total)}
                          </span>
                        </div>
                        <div className="overflow-hidden h-2 text-xs flex rounded bg-gray-200 dark:bg-gray-700">
                          <div
                            style={{ width: `${Math.min(systemMetrics.memory.used_percent, 100)}%` }}
                            className={`shadow-none flex flex-col text-center whitespace-nowrap text-white justify-center transition-all duration-300 ${
                              systemMetrics.memory.used_percent > 80
                                ? 'bg-red-500'
                                : systemMetrics.memory.used_percent > 50
                                ? 'bg-yellow-500'
                                : 'bg-green-500'
                            }`}
                          />
                        </div>
                      </div>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        Свободно: {formatBytes(systemMetrics.memory.free)}
                      </p>
                    </div>

                    {/* Disk */}
                    <div className="space-y-3">
                      <div className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                        <span>💾</span> Диск ({systemMetrics.disk.path})
                      </div>
                      <div className="relative pt-1">
                        <div className="flex mb-2 items-center justify-between">
                          <span className="text-xs font-semibold inline-block text-gray-600 dark:text-gray-400">
                            {systemMetrics.disk.used_percent.toFixed(1)}%
                          </span>
                          <span className="text-xs text-gray-500 dark:text-gray-400">
                            {formatBytes(systemMetrics.disk.used)} / {formatBytes(systemMetrics.disk.total)}
                          </span>
                        </div>
                        <div className="overflow-hidden h-2 text-xs flex rounded bg-gray-200 dark:bg-gray-700">
                          <div
                            style={{ width: `${Math.min(systemMetrics.disk.used_percent, 100)}%` }}
                            className={`shadow-none flex flex-col text-center whitespace-nowrap text-white justify-center transition-all duration-300 ${
                              systemMetrics.disk.used_percent > 90
                                ? 'bg-red-500'
                                : systemMetrics.disk.used_percent > 70
                                ? 'bg-yellow-500'
                                : 'bg-green-500'
                            }`}
                          />
                        </div>
                      </div>
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        Свободно: {formatBytes(systemMetrics.disk.free)}
                      </p>
                    </div>
                  </div>
                ) : (
                  <p className="text-gray-500 dark:text-gray-400">Нет данных</p>
                )}

                {/* Temperature sensors */}
                {systemMetrics && (
                  <div className="mt-6 pt-4 border-t border-gray-200 dark:border-gray-700">
                    <div className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                      <span>🌡️</span> Температура
                    </div>
                    {systemMetrics.temperature && systemMetrics.temperature.length > 0 ? (
                      <div className="flex flex-wrap gap-3">
                        {systemMetrics.temperature.map((temp, idx) => (
                          <div
                            key={idx}
                            className={`px-3 py-2 rounded-lg text-sm ${
                              temp.temperature > 80
                                ? 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'
                                : temp.temperature > 60
                                ? 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400'
                                : 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
                            }`}
                          >
                            <span className="font-medium">{temp.temperature.toFixed(1)}°C</span>
                            <span className="text-xs opacity-75 ml-1">{temp.sensor_key}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-sm text-gray-500 dark:text-gray-400">
                        Датчики температуры недоступны на этой системе (macOS не поддерживает)
                      </p>
                    )}
                  </div>
                )}

                {/* Go runtime info */}
                {systemMetrics && (
                  <div className="mt-6 pt-4 border-t border-gray-200 dark:border-gray-700">
                    <div className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                      <span>🐹</span> Go Runtime
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Версия:</span>
                        <span className="ml-2 font-medium text-gray-900 dark:text-gray-100">{systemMetrics.go.version}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Горутины:</span>
                        <span className="ml-2 font-medium text-gray-900 dark:text-gray-100">{systemMetrics.go.goroutines}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Heap:</span>
                        <span className="ml-2 font-medium text-gray-900 dark:text-gray-100">{formatBytes(systemMetrics.go.heap_alloc)}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">GC:</span>
                        <span className="ml-2 font-medium text-gray-900 dark:text-gray-100">{systemMetrics.go.num_gc} циклов</span>
                      </div>
                    </div>
                  </div>
                )}

                {/* Host info */}
                {systemMetrics && (
                  <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
                    <div className="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
                      <span>🖥️</span> Система
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Хост:</span>
                        <span className="ml-2 font-medium text-gray-900 dark:text-gray-100">{systemMetrics.host.hostname}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">ОС:</span>
                        <span className="ml-2 font-medium text-gray-900 dark:text-gray-100">{systemMetrics.host.platform} {systemMetrics.host.platform_version}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Архитектура:</span>
                        <span className="ml-2 font-medium text-gray-900 dark:text-gray-100">{systemMetrics.host.arch}</span>
                      </div>
                      <div>
                        <span className="text-gray-500 dark:text-gray-400">Uptime:</span>
                        <span className="ml-2 font-medium text-gray-900 dark:text-gray-100">{formatUptime(systemMetrics.host.uptime)}</span>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Queue Actions Card */}
              <div className="card md:col-span-2">
                <h3 className="text-md font-semibold text-gray-900 dark:text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">🛠</span>
                  Управление очередью
                </h3>
                <div className="flex flex-wrap gap-3">
                  <button
                    onClick={handlePurgeInvalidMatches}
                    disabled={isPurging}
                    className="btn btn-secondary"
                  >
                    {isPurging ? 'Очистка...' : 'Удалить невалидные матчи'}
                  </button>
                  <button
                    onClick={handleClearQueue}
                    disabled={isClearing}
                    className="btn btn-danger"
                  >
                    {isClearing ? 'Очистка...' : 'Очистить всю очередь'}
                  </button>
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400 mt-3">
                  «Удалить невалидные матчи» — удаляет из очереди матчи, которые не существуют в базе данных.
                  «Очистить всю очередь» — удаляет все матчи из очереди (требует подтверждения).
                </p>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
