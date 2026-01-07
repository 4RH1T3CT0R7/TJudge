import { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import api from '../api/client';
import { useWebSocket } from '../hooks/useWebSocket';
import { useAuthStore } from '../store/authStore';
import type {
  Tournament,
  TournamentStatus,
  Team,
  Game,
  LeaderboardEntry,
  WSMessage,
  LeaderboardUpdate,
} from '../types';

type TabType = 'info' | 'leaderboard' | 'games' | 'teams';

const statusColors: Record<TournamentStatus, string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  active: 'bg-green-100 text-green-800',
  completed: 'bg-gray-100 text-gray-800',
};

const statusLabels: Record<TournamentStatus, string> = {
  pending: 'Ожидание',
  active: 'Активный',
  completed: 'Завершён',
};

export function TournamentDetail() {
  const { id } = useParams<{ id: string }>();
  const { isAuthenticated, user } = useAuthStore();
  const [tournament, setTournament] = useState<Tournament | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [games, setGames] = useState<Game[]>([]);
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([]);
  const [myTeam, setMyTeam] = useState<Team | null>(null);
  const [activeTab, setActiveTab] = useState<TabType>('info');
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isFullscreen, setIsFullscreen] = useState(false);

  // Join modal state
  const [showJoinModal, setShowJoinModal] = useState(false);
  const [teamName, setTeamName] = useState('');
  const [joinCode, setJoinCode] = useState('');
  const [isJoining, setIsJoining] = useState(false);

  const handleWebSocketMessage = useCallback((message: WSMessage) => {
    if (message.type === 'leaderboard_update') {
      const update = message.payload as LeaderboardUpdate;
      setLeaderboard(update.entries);
    }
  }, []);

  const { isConnected } = useWebSocket({
    tournamentId: id || '',
    onMessage: handleWebSocketMessage,
  });

  useEffect(() => {
    if (id) {
      loadTournamentData();
    }
  }, [id]);

  const loadTournamentData = async () => {
    if (!id) return;

    setIsLoading(true);
    setError(null);

    try {
      // Load tournament data first (required)
      const tournamentData = await api.getTournament(id);
      setTournament(tournamentData);

      // Load other data in parallel, but don't fail if they error
      const [teamsData, gamesData, leaderboardData] = await Promise.all([
        api.getTournamentTeams(id).catch(() => []),
        api.getTournamentGames(id).catch(() => []),
        api.getLeaderboard(id).catch(() => []),
      ]);

      setTeams(teamsData || []);
      setGames(gamesData || []);
      setLeaderboard(leaderboardData || []);

      // Load my team if authenticated
      if (isAuthenticated) {
        try {
          const myTeamData = await api.getMyTeam(id);
          setMyTeam(myTeamData);
        } catch {
          // User might not have a team
          setMyTeam(null);
        }
      }
    } catch (err) {
      setError('Не удалось загрузить данные турнира');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleCreateTeam = async () => {
    if (!id || !teamName.trim()) return;

    setIsJoining(true);
    try {
      const team = await api.createTeam(id, teamName.trim());
      setMyTeam(team);
      setTeams([...teams, team]);
      setShowJoinModal(false);
      setTeamName('');
    } catch (err) {
      console.error('Failed to create team:', err);
    } finally {
      setIsJoining(false);
    }
  };

  const handleJoinTeam = async () => {
    if (!joinCode.trim()) return;

    setIsJoining(true);
    try {
      const team = await api.joinTeamByCode(joinCode.trim());
      setMyTeam(team);
      setShowJoinModal(false);
      setJoinCode('');
      // Reload teams
      if (id) {
        const teamsData = await api.getTournamentTeams(id);
        setTeams(teamsData || []);
      }
    } catch (err) {
      console.error('Failed to join team:', err);
    } finally {
      setIsJoining(false);
    }
  };

  const toggleFullscreen = () => {
    setIsFullscreen(!isFullscreen);
  };

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Загрузка турнира...</p>
      </div>
    );
  }

  if (error || !tournament) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">{error || 'Турнир не найден'}</p>
        <Link to="/tournaments" className="btn btn-secondary mt-4">
          Назад к турнирам
        </Link>
      </div>
    );
  }

  const tabs: { id: TabType; label: string }[] = [
    { id: 'info', label: 'Информация' },
    { id: 'leaderboard', label: 'Таблица' },
    { id: 'games', label: 'Игры' },
    { id: 'teams', label: 'Команды' },
  ];

  const isCreator = user?.id === tournament.creator_id;
  const isAdmin = user?.role === 'admin';
  const canManage = isCreator || isAdmin;
  const canStart = canManage && tournament.status === 'pending';
  const canComplete = canManage && tournament.status === 'active';

  // Fullscreen leaderboard view
  if (isFullscreen) {
    return (
      <div className="fixed inset-0 bg-gray-900 text-white z-50 overflow-auto">
        <div className="p-4">
          <div className="flex justify-between items-center mb-6">
            <h1 className="text-3xl font-bold">{tournament.name} — Таблица лидеров</h1>
            <div className="flex items-center gap-4">
              {isConnected && (
                <span className="flex items-center gap-2">
                  <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></span>
                  Онлайн
                </span>
              )}
              <button onClick={toggleFullscreen} className="btn bg-gray-700 text-white">
                Выйти из полноэкранного режима
              </button>
            </div>
          </div>
          <LeaderboardTable entries={leaderboard} isDark />
        </div>
      </div>
    );
  }

  return (
    <div>
      {/* Header */}
      <div className="flex justify-between items-start mb-6">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h1 className="text-2xl font-bold">{tournament.name}</h1>
            <span className={`px-2 py-1 rounded text-xs font-medium ${statusColors[tournament.status]}`}>
              {statusLabels[tournament.status]}
            </span>
            {tournament.is_permanent && (
              <span className="bg-blue-100 text-blue-800 px-2 py-1 rounded text-xs font-medium">
                Постоянный
              </span>
            )}
          </div>
          <p className="text-gray-600">
            Код: <code className="bg-gray-100 px-2 py-0.5 rounded">{tournament.code}</code>
          </p>
        </div>

        <div className="flex gap-2">
          {isAuthenticated && !myTeam && tournament.status === 'pending' && (
            <button onClick={() => setShowJoinModal(true)} className="btn btn-primary">
              Участвовать
            </button>
          )}
          {canStart && (
            <button
              onClick={async () => {
                await api.startTournament(tournament.id);
                loadTournamentData();
              }}
              className="btn btn-primary"
            >
              Запустить турнир
            </button>
          )}
          {canComplete && (
            <button
              onClick={async () => {
                await api.completeTournament(tournament.id);
                loadTournamentData();
              }}
              className="btn btn-secondary"
            >
              Завершить турнир
            </button>
          )}
        </div>
      </div>

      {/* My Team Badge */}
      {myTeam && (
        <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-lg">
          <p className="text-blue-800">
            Ваша команда: <strong>{myTeam.name}</strong>
            <Link to={`/teams/${myTeam.id}`} className="ml-2 text-blue-600 hover:underline">
              Управление командой
            </Link>
          </p>
        </div>
      )}

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
              {tab.id === 'teams' && ` (${teams.length})`}
              {tab.id === 'games' && ` (${games.length})`}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'info' && (
        <InfoTab tournament={tournament} />
      )}

      {activeTab === 'leaderboard' && (
        <LeaderboardTab
          entries={leaderboard}
          isConnected={isConnected}
          onToggleFullscreen={toggleFullscreen}
        />
      )}

      {activeTab === 'games' && (
        <GamesTab games={games} tournamentId={tournament.id} myTeam={myTeam} />
      )}

      {activeTab === 'teams' && (
        <TeamsTab teams={teams} />
      )}

      {/* Join Modal */}
      {showJoinModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">Участие в турнире</h2>

            <div className="space-y-4">
              <div>
                <h3 className="font-medium mb-2">Создать новую команду</h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={teamName}
                    onChange={(e) => setTeamName(e.target.value)}
                    placeholder="Название команды"
                    className="input flex-1"
                  />
                  <button
                    onClick={handleCreateTeam}
                    disabled={isJoining || !teamName.trim()}
                    className="btn btn-primary"
                  >
                    Создать
                  </button>
                </div>
              </div>

              <div className="border-t pt-4">
                <h3 className="font-medium mb-2">Или присоединиться к существующей</h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={joinCode}
                    onChange={(e) => setJoinCode(e.target.value)}
                    placeholder="Код приглашения"
                    className="input flex-1"
                  />
                  <button
                    onClick={handleJoinTeam}
                    disabled={isJoining || !joinCode.trim()}
                    className="btn btn-secondary"
                  >
                    Вступить
                  </button>
                </div>
              </div>
            </div>

            <button
              onClick={() => setShowJoinModal(false)}
              className="mt-4 w-full btn btn-secondary"
            >
              Отмена
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

// Info Tab Component
function InfoTab({ tournament }: { tournament: Tournament }) {
  return (
    <div className="card">
      {tournament.description ? (
        <div className="prose max-w-none">
          <p>{tournament.description}</p>
        </div>
      ) : (
        <p className="text-gray-500">Описание не указано.</p>
      )}

      <div className="mt-6 grid grid-cols-2 md:grid-cols-4 gap-4">
        <div>
          <p className="text-sm text-gray-500">Макс. размер команды</p>
          <p className="text-lg font-medium">{tournament.max_team_size}</p>
        </div>
        {tournament.max_participants && (
          <div>
            <p className="text-sm text-gray-500">Макс. участников</p>
            <p className="text-lg font-medium">{tournament.max_participants}</p>
          </div>
        )}
        {tournament.start_time && (
          <div>
            <p className="text-sm text-gray-500">Начало</p>
            <p className="text-lg font-medium">
              {new Date(tournament.start_time).toLocaleDateString('ru-RU')}
            </p>
          </div>
        )}
        {tournament.end_time && (
          <div>
            <p className="text-sm text-gray-500">Окончание</p>
            <p className="text-lg font-medium">
              {new Date(tournament.end_time).toLocaleDateString('ru-RU')}
            </p>
          </div>
        )}
        <div>
          <p className="text-sm text-gray-500">Создан</p>
          <p className="text-lg font-medium">
            {new Date(tournament.created_at).toLocaleDateString('ru-RU')}
          </p>
        </div>
      </div>
    </div>
  );
}

// Leaderboard Tab Component
function LeaderboardTab({
  entries,
  isConnected,
  onToggleFullscreen,
}: {
  entries: LeaderboardEntry[];
  isConnected: boolean;
  onToggleFullscreen: () => void;
}) {
  return (
    <div>
      <div className="flex justify-between items-center mb-4">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-semibold">Рейтинг</h2>
          {isConnected && (
            <span className="flex items-center gap-1 text-sm text-green-600">
              <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></span>
              Онлайн
            </span>
          )}
        </div>
        <button onClick={onToggleFullscreen} className="btn btn-secondary">
          На весь экран
        </button>
      </div>
      <LeaderboardTable entries={entries} />
    </div>
  );
}

// Leaderboard Table Component
function LeaderboardTable({
  entries,
  isDark = false,
}: {
  entries: LeaderboardEntry[];
  isDark?: boolean;
}) {
  if (entries.length === 0) {
    return (
      <div className={`text-center py-8 ${isDark ? 'text-gray-400' : 'text-gray-500'}`}>
        Пока нет результатов. Таблица обновится после матчей.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className={`w-full ${isDark ? 'text-white' : ''}`}>
        <thead className={isDark ? 'bg-gray-800' : 'bg-gray-50'}>
          <tr>
            <th className="px-4 py-3 text-left font-medium">Место</th>
            <th className="px-4 py-3 text-left font-medium">Команда / Участник</th>
            <th className="px-4 py-3 text-right font-medium">Рейтинг</th>
            <th className="px-4 py-3 text-right font-medium">П</th>
            <th className="px-4 py-3 text-right font-medium">Пр</th>
            <th className="px-4 py-3 text-right font-medium">Н</th>
            <th className="px-4 py-3 text-right font-medium">Всего</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry, index) => (
            <tr
              key={entry.program_id}
              className={`border-b ${
                isDark
                  ? 'border-gray-700 hover:bg-gray-800'
                  : 'border-gray-200 hover:bg-gray-50'
              }`}
            >
              <td className="px-4 py-3">
                <span
                  className={`inline-flex items-center justify-center w-8 h-8 rounded-full font-bold ${
                    index === 0
                      ? 'bg-yellow-400 text-yellow-900'
                      : index === 1
                      ? 'bg-gray-300 text-gray-800'
                      : index === 2
                      ? 'bg-orange-400 text-orange-900'
                      : isDark
                      ? 'bg-gray-700 text-gray-300'
                      : 'bg-gray-100 text-gray-600'
                  }`}
                >
                  {entry.rank}
                </span>
              </td>
              <td className="px-4 py-3">
                {entry.team_name ? (
                  <span className="font-medium">{entry.team_name}</span>
                ) : entry.username ? (
                  <span className="font-medium">{entry.username}</span>
                ) : (
                  <span className="font-medium">{entry.program_name}</span>
                )}
              </td>
              <td className="px-4 py-3 text-right font-mono font-medium">
                {Math.round(entry.rating)}
              </td>
              <td className="px-4 py-3 text-right text-green-600 font-medium">
                {entry.wins}
              </td>
              <td className="px-4 py-3 text-right text-red-600 font-medium">
                {entry.losses}
              </td>
              <td className="px-4 py-3 text-right text-gray-500 font-medium">
                {entry.draws}
              </td>
              <td className="px-4 py-3 text-right font-mono">
                {entry.total_games}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Games Tab Component
function GamesTab({
  games,
  tournamentId,
  myTeam,
}: {
  games: Game[];
  tournamentId: string;
  myTeam: Team | null;
}) {
  if (games.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        В этот турнир ещё не добавлены игры.
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      {games.map((game) => (
        <Link
          key={game.id}
          to={`/tournaments/${tournamentId}/games/${game.id}`}
          className="card hover:shadow-lg transition-shadow"
        >
          <h3 className="text-lg font-semibold mb-2">{game.display_name}</h3>
          <p className="text-sm text-gray-500 mb-2">
            <code>{game.name}</code>
          </p>
          {game.rules && (
            <p className="text-gray-600 text-sm line-clamp-3">
              {game.rules.substring(0, 200)}...
            </p>
          )}
          {myTeam && (
            <div className="mt-3 text-sm text-primary-600">
              Нажмите для управления вашей программой
            </div>
          )}
        </Link>
      ))}
    </div>
  );
}

// Teams Tab Component
function TeamsTab({ teams }: { teams: Team[] }) {
  if (teams.length === 0) {
    return (
      <div className="text-center py-8 text-gray-500">
        Ни одна команда ещё не присоединилась к турниру.
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {teams.map((team) => (
        <div key={team.id} className="card">
          <h3 className="text-lg font-semibold mb-2">{team.name}</h3>
          <p className="text-sm text-gray-500">
            Присоединились: {new Date(team.created_at).toLocaleDateString('ru-RU')}
          </p>
        </div>
      ))}
    </div>
  );
}
