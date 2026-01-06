import { useState, useEffect, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import api from '../api/client';
import { useAuthStore } from '../store/authStore';
import type { Game, Program, Team } from '../types';

export function GameDetail() {
  const { tournamentId, gameId } = useParams<{ tournamentId: string; gameId: string }>();
  const { isAuthenticated } = useAuthStore();
  const [game, setGame] = useState<Game | null>(null);
  const [myTeam, setMyTeam] = useState<Team | null>(null);
  const [programs, setPrograms] = useState<Program[]>([]);
  const [currentProgram, setCurrentProgram] = useState<Program | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
      setError('Failed to load game data');
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
      setUploadError('Failed to upload program. Please try again.');
    } finally {
      setIsUploading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Loading game...</p>
      </div>
    );
  }

  if (error || !game) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">{error || 'Game not found'}</p>
        <Link to={`/tournaments/${tournamentId}`} className="btn btn-secondary mt-4">
          Back to Tournament
        </Link>
      </div>
    );
  }

  return (
    <div>
      {/* Breadcrumb */}
      <nav className="mb-4 text-sm">
        <Link to="/tournaments" className="text-gray-500 hover:text-gray-700">
          Tournaments
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <Link to={`/tournaments/${tournamentId}`} className="text-gray-500 hover:text-gray-700">
          Tournament
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <span className="text-gray-900">{game.display_name}</span>
      </nav>

      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold mb-2">{game.display_name}</h1>
        <p className="text-gray-500">
          Game ID: <code className="bg-gray-100 px-2 py-0.5 rounded">{game.name}</code>
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Rules Section */}
        <div className="lg:col-span-2">
          <div className="card">
            <h2 className="text-lg font-semibold mb-4">Game Rules</h2>
            {game.rules ? (
              <div className="prose max-w-none">
                <MarkdownRenderer content={game.rules} />
              </div>
            ) : (
              <p className="text-gray-500">No rules provided for this game.</p>
            )}
          </div>
        </div>

        {/* Program Upload Section */}
        <div className="lg:col-span-1">
          {isAuthenticated && myTeam ? (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Your Program</h2>

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
                    Uploaded: {new Date(currentProgram.created_at).toLocaleString()}
                  </p>
                  {currentProgram.error_message && (
                    <div className="mt-2 p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700">
                      <strong>Error:</strong> {currentProgram.error_message}
                    </div>
                  )}
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
                  {isUploading ? 'Uploading...' : currentProgram ? 'Upload New Version' : 'Upload Program'}
                </button>

                {uploadSuccess && (
                  <div className="p-2 bg-green-50 border border-green-200 rounded text-sm text-green-700">
                    Program uploaded successfully!
                  </div>
                )}

                {uploadError && (
                  <div className="p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700">
                    {uploadError}
                  </div>
                )}

                <p className="text-xs text-gray-500">
                  Supported: .py, .cpp, .c, .go, .rs, .java
                </p>
              </div>

              {/* Previous Versions */}
              {programs.length > 1 && (
                <div className="mt-6">
                  <h3 className="font-medium mb-2">Previous Versions</h3>
                  <div className="space-y-2">
                    {programs
                      .filter((p) => p.id !== currentProgram?.id)
                      .sort((a, b) => b.version - a.version)
                      .map((program) => (
                        <div
                          key={program.id}
                          className="flex justify-between items-center text-sm p-2 bg-gray-50 rounded"
                        >
                          <span>v{program.version}</span>
                          <span className="text-gray-500">
                            {new Date(program.created_at).toLocaleDateString()}
                          </span>
                        </div>
                      ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="card">
              <h2 className="text-lg font-semibold mb-4">Submit Program</h2>
              {!isAuthenticated ? (
                <p className="text-gray-500">
                  <Link to="/login" className="text-primary-600 hover:underline">
                    Login
                  </Link>{' '}
                  to submit a program.
                </p>
              ) : (
                <p className="text-gray-500">
                  <Link to={`/tournaments/${tournamentId}`} className="text-primary-600 hover:underline">
                    Join a team
                  </Link>{' '}
                  to submit a program.
                </p>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
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
