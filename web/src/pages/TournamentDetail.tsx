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
      const [tournamentData, teamsData, gamesData, leaderboardData] = await Promise.all([
        api.getTournament(id),
        api.getTournamentTeams(id),
        api.getTournamentGames(id),
        api.getLeaderboard(id),
      ]);

      setTournament(tournamentData);
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
      setError('Failed to load tournament data');
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
        <p className="text-gray-500">Loading tournament...</p>
      </div>
    );
  }

  if (error || !tournament) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">{error || 'Tournament not found'}</p>
        <Link to="/tournaments" className="btn btn-secondary mt-4">
          Back to Tournaments
        </Link>
      </div>
    );
  }

  const tabs: { id: TabType; label: string }[] = [
    { id: 'info', label: 'Info' },
    { id: 'leaderboard', label: 'Leaderboard' },
    { id: 'games', label: 'Games' },
    { id: 'teams', label: 'Teams' },
  ];

  const isCreator = user?.id === tournament.creator_id;
  const canStart = isCreator && tournament.status === 'pending';
  const canComplete = isCreator && tournament.status === 'active';

  // Fullscreen leaderboard view
  if (isFullscreen) {
    return (
      <div className="fixed inset-0 bg-gray-900 text-white z-50 overflow-auto">
        <div className="p-4">
          <div className="flex justify-between items-center mb-6">
            <h1 className="text-3xl font-bold">{tournament.name} - Leaderboard</h1>
            <div className="flex items-center gap-4">
              {isConnected && (
                <span className="flex items-center gap-2">
                  <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></span>
                  Live
                </span>
              )}
              <button onClick={toggleFullscreen} className="btn bg-gray-700 text-white">
                Exit Fullscreen
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
              {tournament.status}
            </span>
            {tournament.is_permanent && (
              <span className="bg-blue-100 text-blue-800 px-2 py-1 rounded text-xs font-medium">
                Permanent
              </span>
            )}
          </div>
          <p className="text-gray-600">
            Code: <code className="bg-gray-100 px-2 py-0.5 rounded">{tournament.code}</code>
          </p>
        </div>

        <div className="flex gap-2">
          {isAuthenticated && !myTeam && tournament.status === 'pending' && (
            <button onClick={() => setShowJoinModal(true)} className="btn btn-primary">
              Join Tournament
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
              Start Tournament
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
              Complete Tournament
            </button>
          )}
        </div>
      </div>

      {/* My Team Badge */}
      {myTeam && (
        <div className="mb-4 p-3 bg-blue-50 border border-blue-200 rounded-lg">
          <p className="text-blue-800">
            Your team: <strong>{myTeam.name}</strong>
            <Link to={`/teams/${myTeam.id}`} className="ml-2 text-blue-600 hover:underline">
              Manage Team
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
            <h2 className="text-xl font-bold mb-4">Join Tournament</h2>

            <div className="space-y-4">
              <div>
                <h3 className="font-medium mb-2">Create a new team</h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={teamName}
                    onChange={(e) => setTeamName(e.target.value)}
                    placeholder="Team name"
                    className="input flex-1"
                  />
                  <button
                    onClick={handleCreateTeam}
                    disabled={isJoining || !teamName.trim()}
                    className="btn btn-primary"
                  >
                    Create
                  </button>
                </div>
              </div>

              <div className="border-t pt-4">
                <h3 className="font-medium mb-2">Or join an existing team</h3>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={joinCode}
                    onChange={(e) => setJoinCode(e.target.value)}
                    placeholder="Team invite code"
                    className="input flex-1"
                  />
                  <button
                    onClick={handleJoinTeam}
                    disabled={isJoining || !joinCode.trim()}
                    className="btn btn-secondary"
                  >
                    Join
                  </button>
                </div>
              </div>
            </div>

            <button
              onClick={() => setShowJoinModal(false)}
              className="mt-4 w-full btn btn-secondary"
            >
              Cancel
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
        <p className="text-gray-500">No description provided.</p>
      )}

      <div className="mt-6 grid grid-cols-2 md:grid-cols-4 gap-4">
        <div>
          <p className="text-sm text-gray-500">Max Team Size</p>
          <p className="text-lg font-medium">{tournament.max_team_size}</p>
        </div>
        {tournament.max_participants && (
          <div>
            <p className="text-sm text-gray-500">Max Participants</p>
            <p className="text-lg font-medium">{tournament.max_participants}</p>
          </div>
        )}
        {tournament.start_time && (
          <div>
            <p className="text-sm text-gray-500">Started</p>
            <p className="text-lg font-medium">
              {new Date(tournament.start_time).toLocaleDateString()}
            </p>
          </div>
        )}
        {tournament.end_time && (
          <div>
            <p className="text-sm text-gray-500">Ended</p>
            <p className="text-lg font-medium">
              {new Date(tournament.end_time).toLocaleDateString()}
            </p>
          </div>
        )}
        <div>
          <p className="text-sm text-gray-500">Created</p>
          <p className="text-lg font-medium">
            {new Date(tournament.created_at).toLocaleDateString()}
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
          <h2 className="text-lg font-semibold">Standings</h2>
          {isConnected && (
            <span className="flex items-center gap-1 text-sm text-green-600">
              <span className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></span>
              Live
            </span>
          )}
        </div>
        <button onClick={onToggleFullscreen} className="btn btn-secondary">
          Fullscreen
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
        No standings yet. Matches will update the leaderboard.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className={`w-full ${isDark ? 'text-white' : ''}`}>
        <thead className={isDark ? 'bg-gray-800' : 'bg-gray-50'}>
          <tr>
            <th className="px-4 py-3 text-left font-medium">Rank</th>
            <th className="px-4 py-3 text-left font-medium">Team / User</th>
            <th className="px-4 py-3 text-right font-medium">Rating</th>
            <th className="px-4 py-3 text-right font-medium">W</th>
            <th className="px-4 py-3 text-right font-medium">L</th>
            <th className="px-4 py-3 text-right font-medium">D</th>
            <th className="px-4 py-3 text-right font-medium">Total</th>
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
                ) : (
                  <span className="font-medium">{entry.username}</span>
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
                {entry.total_matches}
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
        No games added to this tournament yet.
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
              Click to manage your program
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
        No teams have joined this tournament yet.
      </div>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {teams.map((team) => (
        <div key={team.id} className="card">
          <h3 className="text-lg font-semibold mb-2">{team.name}</h3>
          <p className="text-sm text-gray-500">
            Joined: {new Date(team.created_at).toLocaleDateString()}
          </p>
        </div>
      ))}
    </div>
  );
}
