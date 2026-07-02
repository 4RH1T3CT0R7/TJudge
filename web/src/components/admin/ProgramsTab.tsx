import type { Dispatch, SetStateAction } from 'react';
import api from '../../api/client';
import { getGameConfig } from '../../utils/gameConfig';
import type { Game, Tournament, LeaderboardEntry, Program } from '../../types';
import { statusLabels } from './types';
import type { AdminReactionSetter } from './types';

interface ProgramsTabProps {
  tournaments: Tournament[];
  selectedTournamentId: string | null;
  setSelectedTournamentId: Dispatch<SetStateAction<string | null>>;
  tournamentGames: Game[];
  programsData: Record<string, LeaderboardEntry[]>;
  programDetails: Record<string, Program[]>;
  isLoadingPrograms: boolean;
  showLoadingPrograms: boolean;
  setActionError: Dispatch<SetStateAction<string | null>>;
  setAdminReaction: AdminReactionSetter;
}

export function ProgramsTab({
  tournaments,
  selectedTournamentId,
  setSelectedTournamentId,
  tournamentGames,
  programsData,
  programDetails,
  isLoadingPrograms,
  showLoadingPrograms,
  setActionError,
  setAdminReaction,
}: ProgramsTabProps) {
  // Программы выбранного турнира тянет programsQuery по смене selectedTournamentId
  const handleTournamentSelect = (tournamentId: string) => {
    setSelectedTournamentId(tournamentId);
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
      setAdminReaction('handsUp', '// отправляю файл', 2500);
    } catch (err) {
      console.error('Failed to download program:', err);
      setActionError('Не удалось скачать программу');
    }
  };

  // Download all programs as ZIP archive
  const handleDownloadAllPrograms = async () => {
    if (!selectedTournamentId) return;
    try {
      const blob = await api.downloadTournamentPrograms(selectedTournamentId);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `programs_${selectedTournamentId.substring(0, 8)}.zip`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      setAdminReaction('handsUp', '// архивирую...', 2500);
    } catch (err) {
      console.error('Failed to download programs archive:', err);
      setActionError('Не удалось скачать архив программ');
    }
  };

  return (
        <div>
          <h2 className="text-lg font-semibold text-gray-100 mb-4">Просмотр загруженных программ</h2>

          {/* Tournament selector */}
          <div className="mb-6">
            <label className="block text-sm font-medium mb-2 text-gray-300">
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

          {/* Loading state - показываем только после 1s задержки */}
          {showLoadingPrograms && (
            <div className="text-center py-8 text-gray-400">
              Загрузка программ...
            </div>
          )}

          {/* No tournament selected */}
          {!selectedTournamentId && !isLoadingPrograms && (
            <div className="text-center py-8 text-gray-400 bg-gray-800 rounded-lg">
              Выберите турнир для просмотра загруженных программ
            </div>
          )}

          {/* Programs data */}
          {selectedTournamentId && !isLoadingPrograms && !showLoadingPrograms && (
            <div className="space-y-6">
              {/* Total programs count */}
              {tournamentGames.length > 0 && (() => {
                const total = tournamentGames.reduce((sum, game) => {
                  const programs = programsData[game.id] || [];
                  const details = programDetails[game.id] || [];
                  return sum + (programs.length || details.length);
                }, 0);
                return (
                  <div className="flex items-center justify-between">
                    <div className="text-sm text-gray-400">
                      Всего загружено программ: <span className="font-semibold text-gray-200">{total}</span>
                    </div>
                    {total > 0 && (
                      <button
                        onClick={handleDownloadAllPrograms}
                        className="px-3 py-1.5 bg-primary-600 hover:bg-primary-500 text-white text-sm rounded-lg transition-colors"
                      >
                        Скачать все (.zip)
                      </button>
                    )}
                  </div>
                );
              })()}
              {tournamentGames.length === 0 ? (
                <div className="text-center py-8 text-gray-400 bg-gray-800 rounded-lg">
                  В этом турнире нет игр
                </div>
              ) : (
                tournamentGames.map((game) => {
                  const programs = programsData[game.id] || [];
                  const details = programDetails[game.id] || [];
                  const totalPrograms = programs.length || details.length;

                  // Create a lookup map for program errors by team_id
                  // (team_id is used because leaderboard may show older program versions while
                  // programDetails has the latest version - using team_id ensures correct matching)
                  const errorLookup = new Map<string, string>();
                  details.forEach(p => {
                    if (p.error_message && p.team_id) {
                      errorLookup.set(p.team_id, p.error_message);
                    }
                  });

                  // Count programs with errors
                  const programsWithErrors = details.filter(p => p.error_message).length;

                  return (
                    <div key={game.id} className="card">
                      <div className="flex items-center justify-between mb-4">
                        <div className="flex items-center gap-3">
                          <span className="text-2xl">{getGameConfig(game.name).icon}</span>
                          <div>
                            <h3 className="font-semibold text-gray-100">
                              {game.display_name}
                            </h3>
                            <div className="flex items-center gap-2">
                              <p className="text-sm text-gray-400">
                                {totalPrograms} {totalPrograms === 1 ? 'программа' : totalPrograms < 5 ? 'программы' : 'программ'}
                              </p>
                              {programsWithErrors > 0 && (
                                <span className="px-2 py-0.5 bg-red-900/30 text-red-400 text-xs rounded-full">
                                  {programsWithErrors} с ошибкой
                                </span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>

                      {programs.length === 0 && details.length === 0 ? (
                        <p className="text-sm text-gray-400">
                          Программы ещё не загружены
                        </p>
                      ) : programs.length > 0 ? (
                        <div className="overflow-x-auto">
                          <table className="w-full">
                            <thead>
                              <tr className="text-left text-sm text-gray-400 border-b border-gray-700">
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
                                const error = entry.team_id ? errorLookup.get(entry.team_id) : undefined;
                                return (
                                  <tr key={entry.program_id} className="border-b border-gray-800">
                                    <td className="py-2 pr-4 font-medium text-gray-400">{entry.rank}</td>
                                    <td className="py-2 pr-4">
                                      <div className="font-medium text-gray-100">
                                        {entry.program_name}
                                      </div>
                                      <code className="text-xs text-gray-500 font-mono">
                                        {entry.program_id.substring(0, 8)}...
                                      </code>
                                    </td>
                                    <td className="py-2 pr-4 text-gray-300">
                                      {entry.team_name || '-'}
                                    </td>
                                    <td className="py-2 pr-4 text-center font-bold text-gray-100">
                                      {entry.rating}
                                    </td>
                                    <td className="py-2 pr-4 text-center text-green-400">
                                      {entry.wins}
                                    </td>
                                    <td className="py-2 pr-4 text-center text-red-400">
                                      {entry.losses}
                                    </td>
                                    <td className="py-2 pr-4 text-center text-gray-400">
                                      {entry.draws}
                                    </td>
                                    <td className="py-2 pr-4 text-center text-gray-300">
                                      {entry.total_games}
                                    </td>
                                    <td className="py-2 pr-4">
                                      {error ? (
                                        <div className="group relative">
                                          <span className="px-2 py-1 bg-red-900/30 text-red-400 text-xs rounded cursor-help">
                                            Ошибка
                                          </span>
                                          <div className="absolute z-10 hidden group-hover:block w-80 p-2 bg-gray-900 text-white text-xs rounded shadow-lg -left-32 top-full mt-1">
                                            <pre className="whitespace-pre-wrap break-words font-mono">{error}</pre>
                                          </div>
                                        </div>
                                      ) : (
                                        <span className="px-2 py-1 bg-green-900/30 text-green-400 text-xs rounded">
                                          OK
                                        </span>
                                      )}
                                    </td>
                                    <td className="py-2">
                                      <button
                                        onClick={() => handleDownloadProgram(entry.program_id, entry.program_name)}
                                        className="text-primary-400 hover:text-primary-300 text-sm"
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
                              <tr className="text-left text-sm text-gray-400 border-b border-gray-700">
                                <th className="pb-2 pr-4">Программа</th>
                                <th className="pb-2 pr-4">Версия</th>
                                <th className="pb-2 pr-4">Язык</th>
                                <th className="pb-2 pr-4">Статус</th>
                                <th className="pb-2">Действия</th>
                              </tr>
                            </thead>
                            <tbody>
                              {details.map((prog) => (
                                <tr key={prog.id} className="border-b border-gray-800">
                                  <td className="py-2 pr-4">
                                    <div className="font-medium text-gray-100">
                                      {prog.name}
                                    </div>
                                    <code className="text-xs text-gray-500 font-mono">
                                      {prog.id.substring(0, 8)}...
                                    </code>
                                  </td>
                                  <td className="py-2 pr-4 text-gray-300">
                                    v{prog.version}
                                  </td>
                                  <td className="py-2 pr-4 text-gray-300">
                                    {prog.language}
                                  </td>
                                  <td className="py-2 pr-4">
                                    {prog.error_message ? (
                                      <div className="group relative">
                                        <span className="px-2 py-1 bg-red-900/30 text-red-400 text-xs rounded cursor-help">
                                          Ошибка
                                        </span>
                                        <div className="absolute z-10 hidden group-hover:block w-80 p-2 bg-gray-900 text-white text-xs rounded shadow-lg -left-32 top-full mt-1">
                                          <pre className="whitespace-pre-wrap break-words font-mono">{prog.error_message}</pre>
                                        </div>
                                      </div>
                                    ) : (
                                      <span className="px-2 py-1 bg-green-900/30 text-green-400 text-xs rounded">
                                        OK
                                      </span>
                                    )}
                                  </td>
                                  <td className="py-2">
                                    <button
                                      onClick={() => handleDownloadProgram(prog.id, prog.name)}
                                      className="text-primary-400 hover:text-primary-300 text-sm"
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
  );
}
