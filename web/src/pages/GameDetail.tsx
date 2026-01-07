import { useState, useEffect, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import api from '../api/client';
import { useAuthStore } from '../store/authStore';
import type { Game, Program, Team, LeaderboardEntry, Match } from '../types';

export function GameDetail() {
  const { tournamentId, gameId } = useParams<{ tournamentId: string; gameId: string }>();
  const { isAuthenticated } = useAuthStore();
  const [game, setGame] = useState<Game | null>(null);
  const [myTeam, setMyTeam] = useState<Team | null>(null);
  const [programs, setPrograms] = useState<Program[]>([]);
  const [currentProgram, setCurrentProgram] = useState<Program | null>(null);
  const [leaderboard, setLeaderboard] = useState<LeaderboardEntry[]>([]);
  const [matches, setMatches] = useState<Match[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'rules' | 'leaderboard' | 'matches'>('rules');

  // Upload state
  const [isUploading, setIsUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [uploadSuccess, setUploadSuccess] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (gameId && tournamentId) {
      loadData();
    }
  }, [gameId, tournamentId]);

  const loadData = async () => {
    if (!gameId || !tournamentId) return;

    setIsLoading(true);
    setError(null);

    try {
      const gameData = await api.getGame(gameId);
      setGame(gameData);

      // Load leaderboard and matches in parallel
      const [leaderboardData, matchesData] = await Promise.all([
        api.getGameLeaderboard(tournamentId, gameId).catch(() => []),
        api.getGameMatches(tournamentId, gameId).catch(() => []),
      ]);
      setLeaderboard(leaderboardData || []);
      setMatches(matchesData || []);

      if (isAuthenticated) {
        try {
          const teamData = await api.getMyTeam(tournamentId);
          setMyTeam(teamData);

          // Load programs for this team
          const programsData = await api.getPrograms();
          const teamPrograms = programsData.filter(
            (p) => p.team_id === teamData?.id && p.game_id === gameId
          );
          setPrograms(teamPrograms);

          // Set current program (latest version)
          if (teamPrograms.length > 0) {
            const latest = teamPrograms.reduce((a, b) =>
              a.version > b.version ? a : b
            );
            setCurrentProgram(latest);
          }
        } catch {
          // User might not have a team
        }
      }
    } catch (err) {
      setError('Не удалось загрузить данные игры');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleFileSelect = () => {
    fileInputRef.current?.click();
  };

  const handleFileUpload = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file || !tournamentId || !gameId || !myTeam) return;

    setIsUploading(true);
    setUploadError(null);
    setUploadSuccess(false);

    try {
      const formData = new FormData();
      formData.append('file', file);
      formData.append('team_id', myTeam.id);
      formData.append('tournament_id', tournamentId);
      formData.append('game_id', gameId);
      formData.append('name', file.name);

      const program = await api.uploadProgram(formData);
      setCurrentProgram(program);
      setPrograms([...programs, program]);
      setUploadSuccess(true);

      // Clear file input
      if (fileInputRef.current) {
        fileInputRef.current.value = '';
      }

      // Hide success message after 3 seconds
      setTimeout(() => setUploadSuccess(false), 3000);
    } catch (err) {
      console.error('Upload failed:', err);
      setUploadError('Не удалось загрузить программу. Попробуйте снова.');
    } finally {
      setIsUploading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Загрузка игры...</p>
      </div>
    );
  }

  if (error || !game) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">{error || 'Игра не найдена'}</p>
        <Link to={`/tournaments/${tournamentId}`} className="btn btn-secondary mt-4">
          Назад к турниру
        </Link>
      </div>
    );
  }

  return (
    <div>
      {/* Breadcrumb */}
      <nav className="mb-4 text-sm">
        <Link to="/tournaments" className="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300">
          Турниры
        </Link>
        <span className="mx-2 text-gray-400 dark:text-gray-600">/</span>
        <Link to={`/tournaments/${tournamentId}`} className="text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300">
          Турнир
        </Link>
        <span className="mx-2 text-gray-400 dark:text-gray-600">/</span>
        <span className="text-gray-900 dark:text-gray-200">{game.display_name}</span>
      </nav>

      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold mb-2 dark:text-gray-100">{game.display_name}</h1>
        <p className="text-gray-500 dark:text-gray-400">
          ID игры: <code className="bg-gray-800 dark:bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{game.name}</code>
        </p>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200 dark:border-gray-700 mb-6">
        <nav className="-mb-px flex space-x-8">
          <button
            onClick={() => setActiveTab('rules')}
            className={`py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === 'rules'
                ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
            }`}
          >
            Правила
          </button>
          <button
            onClick={() => setActiveTab('leaderboard')}
            className={`py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === 'leaderboard'
                ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
            }`}
          >
            Рейтинг ({leaderboard.length})
          </button>
          <button
            onClick={() => setActiveTab('matches')}
            className={`py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === 'matches'
                ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 hover:border-gray-300 dark:hover:border-gray-600'
            }`}
          >
            Матчи ({matches.length})
          </button>
        </nav>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main Content Section */}
        <div className="lg:col-span-2">
          {activeTab === 'rules' && (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Правила игры</h2>
              {game.rules ? (
                <div className="prose max-w-none">
                  <MarkdownRenderer content={game.rules} />
                </div>
              ) : (
                <p className="text-gray-500">Правила для этой игры не указаны.</p>
              )}
            </div>
          )}

          {activeTab === 'leaderboard' && (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Таблица рейтинга</h2>
              {leaderboard.length > 0 ? (
                <div className="overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className="text-left text-sm text-gray-500 border-b">
                        <th className="pb-2 pr-4">#</th>
                        <th className="pb-2 pr-4">Программа</th>
                        <th className="pb-2 pr-4 text-center">Рейтинг</th>
                        <th className="pb-2 pr-4 text-center">W</th>
                        <th className="pb-2 pr-4 text-center">L</th>
                        <th className="pb-2 pr-4 text-center">D</th>
                        <th className="pb-2 text-center">Игр</th>
                      </tr>
                    </thead>
                    <tbody>
                      {leaderboard.map((entry) => (
                        <tr key={entry.program_id} className="border-b border-gray-100">
                          <td className="py-2 pr-4 font-medium">{entry.rank}</td>
                          <td className="py-2 pr-4">{entry.program_name}</td>
                          <td className="py-2 pr-4 text-center font-medium">{entry.rating}</td>
                          <td className="py-2 pr-4 text-center text-green-600">{entry.wins}</td>
                          <td className="py-2 pr-4 text-center text-red-600">{entry.losses}</td>
                          <td className="py-2 pr-4 text-center text-gray-500">{entry.draws}</td>
                          <td className="py-2 text-center">{entry.total_games}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="text-gray-500">Нет данных рейтинга. Загрузите программу и дождитесь результатов матчей.</p>
              )}
            </div>
          )}

          {activeTab === 'matches' && (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4 dark:text-gray-100">Результаты матчей</h2>
              {matches.length > 0 ? (
                <MatchGroups matches={matches} />
              ) : (
                <p className="text-gray-500 dark:text-gray-400">Матчи ещё не проводились.</p>
              )}
            </div>
          )}
        </div>

        {/* Program Upload Section */}
        <div className="lg:col-span-1">
          {isAuthenticated && myTeam ? (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Ваша программа</h2>

              {/* Current Program */}
              {currentProgram && (
                <div className="mb-4 p-3 bg-gray-50 rounded-lg">
                  <div className="flex justify-between items-start mb-2">
                    <p className="font-medium">{currentProgram.name}</p>
                    <span className="text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded">
                      v{currentProgram.version}
                    </span>
                  </div>
                  <p className="text-sm text-gray-500">
                    Загружена: {new Date(currentProgram.created_at).toLocaleString('ru-RU')}
                  </p>
                  {currentProgram.error_message && (
                    <div className="mt-2 p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700">
                      <strong>Ошибка:</strong> {currentProgram.error_message}
                    </div>
                  )}
                  <button
                    onClick={async () => {
                      try {
                        const blob = await api.downloadProgram(currentProgram.id);
                        const url = window.URL.createObjectURL(blob);
                        const a = document.createElement('a');
                        a.href = url;
                        a.download = currentProgram.name || 'program';
                        document.body.appendChild(a);
                        a.click();
                        window.URL.revokeObjectURL(url);
                        document.body.removeChild(a);
                      } catch (err) {
                        console.error('Download failed:', err);
                        alert('Не удалось скачать программу');
                      }
                    }}
                    className="btn btn-secondary w-full mt-2 text-sm"
                  >
                    Скачать программу
                  </button>
                </div>
              )}

              {/* Upload Form */}
              <div className="space-y-3">
                <input
                  type="file"
                  ref={fileInputRef}
                  onChange={handleFileUpload}
                  className="hidden"
                  accept=".py,.cpp,.c,.go,.rs,.java"
                />
                <button
                  onClick={handleFileSelect}
                  disabled={isUploading}
                  className="btn btn-primary w-full"
                >
                  {isUploading ? 'Загрузка...' : currentProgram ? 'Загрузить новую версию' : 'Загрузить программу'}
                </button>

                {uploadSuccess && (
                  <div className="p-2 bg-green-50 border border-green-200 rounded text-sm text-green-700">
                    Программа успешно загружена!
                  </div>
                )}

                {uploadError && (
                  <div className="p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700">
                    {uploadError}
                  </div>
                )}

                <p className="text-xs text-gray-500">
                  Поддерживаемые форматы: .py, .cpp, .c, .go, .rs, .java
                </p>
              </div>

              {/* Previous Versions */}
              {programs.length > 1 && (
                <div className="mt-6">
                  <h3 className="font-medium mb-2 dark:text-gray-100">Предыдущие версии</h3>
                  <div className="space-y-2">
                    {programs
                      .filter((p) => p.id !== currentProgram?.id)
                      .sort((a, b) => b.version - a.version)
                      .map((program) => (
                        <div
                          key={program.id}
                          className="flex justify-between items-center text-sm p-2 bg-gray-50 dark:bg-gray-800 rounded"
                        >
                          <div className="flex flex-col">
                            <span className="dark:text-gray-100">v{program.version}</span>
                            <span className="text-xs text-gray-500 dark:text-gray-400">
                              {new Date(program.created_at).toLocaleDateString('ru-RU')}
                            </span>
                          </div>
                          <button
                            onClick={async () => {
                              try {
                                const blob = await api.downloadProgram(program.id);
                                const url = window.URL.createObjectURL(blob);
                                const a = document.createElement('a');
                                a.href = url;
                                a.download = program.name || `program_v${program.version}`;
                                document.body.appendChild(a);
                                a.click();
                                window.URL.revokeObjectURL(url);
                                document.body.removeChild(a);
                              } catch (err) {
                                console.error('Download failed:', err);
                                alert('Не удалось скачать программу');
                              }
                            }}
                            className="text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300 text-xs font-medium"
                          >
                            Скачать
                          </button>
                        </div>
                      ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Отправить программу</h2>
              {!isAuthenticated ? (
                <p className="text-gray-500">
                  <Link to="/login" className="text-primary-600 hover:underline">
                    Войдите
                  </Link>{' '}
                  чтобы отправить программу.
                </p>
              ) : (
                <p className="text-gray-500">
                  <Link to={`/tournaments/${tournamentId}`} className="text-primary-600 hover:underline">
                    Присоединитесь к команде
                  </Link>{' '}
                  чтобы отправить программу.
                </p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

// Match groups component - groups matches by program pair and shows iterations as tabs
function MatchGroups({ matches }: { matches: Match[] }) {
  // Group matches by program pair (program1_id + program2_id)
  const groupedMatches: Record<string, Match[]> = {};

  matches.forEach((match) => {
    // Create a consistent key regardless of which program is first
    const ids = [match.program1_id, match.program2_id].sort();
    const key = ids.join('-');

    if (!groupedMatches[key]) {
      groupedMatches[key] = [];
    }
    groupedMatches[key].push(match);
  });

  // Sort matches within each group by created_at
  Object.values(groupedMatches).forEach((group) => {
    group.sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
  });

  const groupEntries = Object.entries(groupedMatches);

  if (groupEntries.length === 0) {
    return <p className="text-gray-500 dark:text-gray-400">Матчи ещё не проводились.</p>;
  }

  return (
    <div className="space-y-4">
      {groupEntries.map(([key, groupMatches]) => (
        <MatchGroupCard key={key} matches={groupMatches} />
      ))}
    </div>
  );
}

// Match group card with iteration tabs
function MatchGroupCard({ matches }: { matches: Match[] }) {
  const [activeIteration, setActiveIteration] = useState(0);
  const activeMatch = matches[activeIteration];

  // Calculate aggregate stats
  const stats = {
    completed: matches.filter((m) => m.status === 'completed').length,
    pending: matches.filter((m) => m.status === 'pending').length,
    running: matches.filter((m) => m.status === 'running').length,
    failed: matches.filter((m) => m.status === 'failed').length,
    total1: matches.reduce((sum, m) => sum + (m.score1 ?? 0), 0),
    total2: matches.reduce((sum, m) => sum + (m.score2 ?? 0), 0),
    wins1: matches.filter((m) => m.winner === 1).length,
    wins2: matches.filter((m) => m.winner === 2).length,
    draws: matches.filter((m) => m.winner === 0).length,
  };

  const getIterationStatus = (match: Match) => {
    switch (match.status) {
      case 'completed':
        if (match.winner === 1) return 'bg-green-500';
        if (match.winner === 2) return 'bg-red-500';
        return 'bg-gray-400';
      case 'running':
        return 'bg-blue-500 animate-pulse';
      case 'failed':
        return 'bg-red-600';
      default:
        return 'bg-gray-300 dark:bg-gray-600';
    }
  };

  return (
    <div className="bg-gray-50 dark:bg-gray-800/50 rounded-lg p-4 border border-gray-200 dark:border-gray-700">
      {/* Header with aggregate stats */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-4 gap-2">
        <div className="flex items-center gap-4">
          <div className="text-center">
            <p className="text-sm font-medium text-gray-600 dark:text-gray-400 truncate max-w-[120px]" title={matches[0]?.program1_id}>
              Программа 1
            </p>
            <p className="text-2xl font-bold text-gray-900 dark:text-gray-100">{stats.total1}</p>
          </div>
          <div className="text-center text-gray-400 dark:text-gray-500">
            <span className="text-lg">vs</span>
          </div>
          <div className="text-center">
            <p className="text-sm font-medium text-gray-600 dark:text-gray-400 truncate max-w-[120px]" title={matches[0]?.program2_id}>
              Программа 2
            </p>
            <p className="text-2xl font-bold text-gray-900 dark:text-gray-100">{stats.total2}</p>
          </div>
        </div>

        <div className="flex items-center gap-3 text-sm">
          <span className="text-green-600 dark:text-green-400" title="Победы 1">
            W1: {stats.wins1}
          </span>
          <span className="text-gray-500 dark:text-gray-400" title="Ничьи">
            D: {stats.draws}
          </span>
          <span className="text-red-600 dark:text-red-400" title="Победы 2">
            W2: {stats.wins2}
          </span>
          <span className="text-gray-400 dark:text-gray-500">
            ({stats.completed}/{matches.length})
          </span>
        </div>
      </div>

      {/* Iteration tabs */}
      <div className="mb-3">
        <div className="flex flex-wrap gap-1.5">
          {matches.map((match, index) => (
            <button
              key={match.id}
              onClick={() => setActiveIteration(index)}
              className={`w-8 h-8 rounded-lg text-xs font-medium transition-all ${
                activeIteration === index
                  ? 'ring-2 ring-primary-500 ring-offset-1 dark:ring-offset-gray-800'
                  : 'hover:scale-105'
              }`}
              title={`Итерация ${index + 1}: ${match.status}${match.winner !== undefined ? ` (Победа ${match.winner || 'Ничья'})` : ''}`}
            >
              <div className={`w-full h-full rounded-lg flex items-center justify-center text-white ${getIterationStatus(match)}`}>
                {index + 1}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Active iteration details */}
      {activeMatch && (
        <div className="bg-white dark:bg-gray-800 rounded-lg p-3 border border-gray-200 dark:border-gray-600">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
              Итерация {activeIteration + 1}
            </span>
            <MatchStatusBadge status={activeMatch.status} />
          </div>

          <div className="flex items-center justify-center gap-4 py-2">
            <span className={`text-xl font-bold ${activeMatch.winner === 1 ? 'text-green-600 dark:text-green-400' : 'text-gray-700 dark:text-gray-300'}`}>
              {activeMatch.score1 ?? '-'}
            </span>
            <span className="text-gray-400">:</span>
            <span className={`text-xl font-bold ${activeMatch.winner === 2 ? 'text-green-600 dark:text-green-400' : 'text-gray-700 dark:text-gray-300'}`}>
              {activeMatch.score2 ?? '-'}
            </span>
          </div>

          {activeMatch.winner !== undefined && activeMatch.status === 'completed' && (
            <p className="text-center text-sm text-gray-500 dark:text-gray-400">
              {activeMatch.winner === 0 ? 'Ничья' : `Победа Программы ${activeMatch.winner}`}
            </p>
          )}

          {activeMatch.error_message && (
            <div className="mt-2 p-2 bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-700 rounded text-xs text-red-700 dark:text-red-300">
              {activeMatch.error_message}
            </div>
          )}

          <p className="text-xs text-gray-400 dark:text-gray-500 mt-2 text-center">
            {new Date(activeMatch.created_at).toLocaleString('ru-RU')}
          </p>
        </div>
      )}
    </div>
  );
}

// Match status badge component
function MatchStatusBadge({ status }: { status: string }) {
  switch (status) {
    case 'pending':
      return <span className="text-xs bg-yellow-100 dark:bg-yellow-900/50 text-yellow-800 dark:text-yellow-300 px-2 py-0.5 rounded">Ожидание</span>;
    case 'running':
      return <span className="text-xs bg-blue-100 dark:bg-blue-900/50 text-blue-800 dark:text-blue-300 px-2 py-0.5 rounded">Выполняется</span>;
    case 'completed':
      return <span className="text-xs bg-green-100 dark:bg-green-900/50 text-green-800 dark:text-green-300 px-2 py-0.5 rounded">Завершён</span>;
    case 'failed':
      return <span className="text-xs bg-red-100 dark:bg-red-900/50 text-red-800 dark:text-red-300 px-2 py-0.5 rounded">Ошибка</span>;
    default:
      return null;
  }
}

// Simple Markdown renderer (for basic formatting)
function MarkdownRenderer({ content }: { content: string }) {
  // Basic markdown parsing for common patterns
  const parseMarkdown = (text: string): string => {
    let html = text
      // Escape HTML
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      // Headers
      .replace(/^### (.*$)/gim, '<h3 class="text-lg font-semibold mt-4 mb-2">$1</h3>')
      .replace(/^## (.*$)/gim, '<h2 class="text-xl font-semibold mt-6 mb-3">$1</h2>')
      .replace(/^# (.*$)/gim, '<h1 class="text-2xl font-bold mt-6 mb-4">$1</h1>')
      // Bold
      .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      // Italic
      .replace(/\*(.*?)\*/g, '<em>$1</em>')
      // Code blocks
      .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre class="bg-gray-100 p-3 rounded-lg overflow-x-auto my-3"><code>$2</code></pre>')
      // Inline code
      .replace(/`([^`]+)`/g, '<code class="bg-gray-100 px-1 py-0.5 rounded text-sm">$1</code>')
      // Links
      .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" class="text-primary-600 hover:underline" target="_blank" rel="noopener noreferrer">$1</a>')
      // Unordered lists
      .replace(/^\s*[-*] (.*$)/gim, '<li class="ml-4">$1</li>')
      // Ordered lists
      .replace(/^\s*\d+\. (.*$)/gim, '<li class="ml-4 list-decimal">$1</li>')
      // Paragraphs (double newline)
      .replace(/\n\n/g, '</p><p class="my-3">')
      // Single newlines to <br>
      .replace(/\n/g, '<br />');

    // Wrap in paragraph tags
    html = '<p class="my-3">' + html + '</p>';

    return html;
  };

  return (
    <div
      className="markdown-content"
      dangerouslySetInnerHTML={{ __html: parseMarkdown(content) }}
    />
  );
}
