import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../api/client';
import { useAuthStore } from '../store/authStore';
import type { Game, Tournament } from '../types';

type AdminTab = 'games' | 'tournaments';

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
    max_team_size: 3,
    is_permanent: false,
  });
  const [isSavingTournament, setIsSavingTournament] = useState(false);

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
      setGameError('Name and display name are required');
      return;
    }

    // Validate name format
    if (!/^[a-z0-9_]+$/.test(gameForm.name)) {
      setGameError('Name must contain only lowercase letters, numbers, and underscores');
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
      setGameError('Failed to save game');
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
    if (!tournamentForm.name.trim()) return;

    setIsSavingTournament(true);
    try {
      const newTournament = await api.createTournament(tournamentForm);
      setTournaments([...tournaments, newTournament]);
      resetTournamentForm();
    } catch (err) {
      console.error('Failed to create tournament:', err);
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
      max_team_size: 3,
      is_permanent: false,
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

  if (user?.role !== 'admin') {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">Access denied. Admin privileges required.</p>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Loading...</p>
      </div>
    );
  }

  const tabs: { id: AdminTab; label: string }[] = [
    { id: 'games', label: `Games (${games.length})` },
    { id: 'tournaments', label: `Tournaments (${tournaments.length})` },
  ];

  return (
    <div>
      <h1 className="text-2xl font-bold mb-6">Admin Panel</h1>

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
            <h2 className="text-lg font-semibold">Manage Games</h2>
            <button onClick={() => setShowGameForm(true)} className="btn btn-primary">
              Add Game
            </button>
          </div>

          {/* Game Form Modal */}
          {showGameForm && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-white rounded-lg p-6 w-full max-w-2xl max-h-[90vh] overflow-y-auto">
                <h2 className="text-xl font-bold mb-4">
                  {editingGame ? 'Edit Game' : 'Create New Game'}
                </h2>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1">
                      Name (unique identifier)
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
                      Only lowercase letters, numbers, and underscores
                    </p>
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Display Name</label>
                    <input
                      type="text"
                      value={gameForm.display_name}
                      onChange={(e) =>
                        setGameForm({ ...gameForm, display_name: e.target.value })
                      }
                      className="input"
                      placeholder="Game Name"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Rules (Markdown)</label>
                    <textarea
                      value={gameForm.rules}
                      onChange={(e) => setGameForm({ ...gameForm, rules: e.target.value })}
                      className="input min-h-[200px] font-mono text-sm"
                      placeholder="# Game Rules&#10;&#10;Write rules in Markdown format..."
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
                    Cancel
                  </button>
                  <button
                    onClick={handleCreateGame}
                    disabled={isSavingGame}
                    className="btn btn-primary"
                  >
                    {isSavingGame ? 'Saving...' : editingGame ? 'Update' : 'Create'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Games List */}
          {games.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              No games created yet.
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
                      Edit
                    </button>
                    {deleteGameId === game.id ? (
                      <div className="flex gap-1">
                        <button
                          onClick={() => handleDeleteGame(game.id)}
                          className="btn btn-danger text-sm"
                        >
                          Confirm
                        </button>
                        <button
                          onClick={() => setDeleteGameId(null)}
                          className="btn btn-secondary text-sm"
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => setDeleteGameId(game.id)}
                        className="btn btn-danger text-sm"
                      >
                        Delete
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
            <h2 className="text-lg font-semibold">Manage Tournaments</h2>
            <button onClick={() => setShowTournamentForm(true)} className="btn btn-primary">
              Create Tournament
            </button>
          </div>

          {/* Tournament Form Modal */}
          {showTournamentForm && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-white rounded-lg p-6 w-full max-w-md">
                <h2 className="text-xl font-bold mb-4">Create Tournament</h2>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1">Name</label>
                    <input
                      type="text"
                      value={tournamentForm.name}
                      onChange={(e) =>
                        setTournamentForm({ ...tournamentForm, name: e.target.value })
                      }
                      className="input"
                      placeholder="Tournament Name"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Description</label>
                    <textarea
                      value={tournamentForm.description}
                      onChange={(e) =>
                        setTournamentForm({ ...tournamentForm, description: e.target.value })
                      }
                      className="input min-h-[100px]"
                      placeholder="Tournament description..."
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Max Team Size</label>
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
                      Permanent tournament (always accepting new participants)
                    </label>
                  </div>
                </div>

                <div className="flex justify-end gap-2 mt-6">
                  <button onClick={resetTournamentForm} className="btn btn-secondary">
                    Cancel
                  </button>
                  <button
                    onClick={handleCreateTournament}
                    disabled={isSavingTournament || !tournamentForm.name.trim()}
                    className="btn btn-primary"
                  >
                    {isSavingTournament ? 'Creating...' : 'Create'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Tournaments List */}
          {tournaments.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              No tournaments created yet.
            </div>
          ) : (
            <div className="space-y-4">
              {tournaments.map((tournament) => (
                <div key={tournament.id} className="card">
                  <div className="flex justify-between items-start">
                    <div>
                      <h3 className="font-semibold">{tournament.name}</h3>
                      <p className="text-sm text-gray-500">
                        Code: <code>{tournament.code}</code>
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
                        {tournament.status}
                      </span>
                      {tournament.is_permanent && (
                        <span className="bg-blue-100 text-blue-800 px-2 py-1 rounded text-xs font-medium">
                          Permanent
                        </span>
                      )}
                    </div>
                  </div>
                  <div className="mt-3 flex gap-2">
                    <a
                      href={`/tournaments/${tournament.id}`}
                      className="btn btn-secondary text-sm"
                    >
                      View
                    </a>
                    {tournament.status === 'pending' && (
                      <button
                        onClick={async () => {
                          await api.startTournament(tournament.id);
                          loadData();
                        }}
                        className="btn btn-primary text-sm"
                      >
                        Start
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
                        Complete
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
