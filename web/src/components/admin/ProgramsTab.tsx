import type { Dispatch, SetStateAction } from 'react';
import api from '../../api/client';
import { getGameConfig } from '../../utils/gameConfig';
import type { Game, Tournament, Program } from '../../types';
import { statusLabels } from './types';
import type { AdminReactionSetter } from './types';
import { Field } from '../ui/Field';
import { StatusLabel } from '../ui/StatusLabel';
import { Spinner } from '../ui/Spinner';
import { EmptyState } from '../ui/EmptyState';
import type { ProgramRow } from './programRows';

const playingLabel: Record<ProgramRow['playing'], string> = {
  this: 'да',
  previous: 'предыдущая версия',
  none: 'нет',
};

interface ProgramsTabProps {
  tournaments: Tournament[];
  selectedTournamentId: string | null;
  setSelectedTournamentId: Dispatch<SetStateAction<string | null>>;
  tournamentGames: Game[];
  programRows: Record<string, ProgramRow[]>;
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
  programRows,
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
  const handleDownloadProgram = async (program: Program) => {
    try {
      const blob = await api.downloadProgram(program.id);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      // имя загруженного файла, с его расширением
      a.download = program.name || `program_v${program.version}`;
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
          <Field label="Выберите турнир" className="mb-6">
            {(control) => (
              <select
                {...control}
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
            )}
          </Field>

          {/* Loading state - показываем только после 1s задержки */}
          {showLoadingPrograms && (
            <div className="flex justify-center py-8 text-sm text-gray-400">
              <Spinner>загрузка программ</Spinner>
            </div>
          )}

          {/* No tournament selected */}
          {!selectedTournamentId && !isLoadingPrograms && (
            <EmptyState command="программы" hint="выберите турнир для просмотра загруженных программ" />
          )}

          {/* Programs data */}
          {selectedTournamentId && !isLoadingPrograms && !showLoadingPrograms && (
            <div className="space-y-6">
              {/* Total programs count */}
              {tournamentGames.length > 0 && (() => {
                const total = tournamentGames.reduce((sum, game) => sum + (programRows[game.id]?.length ?? 0), 0);
                return (
                  <div className="flex flex-wrap items-center justify-between gap-2">
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
                <EmptyState command="игры" hint="в этом турнире нет игр" />
              ) : (
                tournamentGames.map((game) => {
                  const rows = programRows[game.id] ?? [];
                  const failedCount = rows.filter((r) => r.program.status === 'failed').length;

                  return (
                    <div key={game.id} className="card">
                      <div className="flex items-center justify-between mb-4">
                        <div className="flex items-center gap-3">
                          <span aria-hidden="true" className={`font-mono text-2xl ${getGameConfig(game.name).textClass}`}>{getGameConfig(game.name).icon}</span>
                          <div>
                            <h3 className="font-semibold text-gray-100">
                              {game.display_name}
                            </h3>
                            <div className="flex items-center gap-2">
                              <p className="text-sm text-gray-400">
                                {rows.length} {rows.length === 1 ? 'программа' : rows.length > 1 && rows.length < 5 ? 'программы' : 'программ'}
                              </p>
                              {failedCount > 0 && (
                                <span className="px-2 py-0.5 bg-red-900/30 text-red-400 text-xs rounded-full">
                                  {failedCount} с ошибкой сборки
                                </span>
                              )}
                            </div>
                          </div>
                        </div>
                      </div>

                      {rows.length === 0 ? (
                        <p className="text-sm text-gray-400">
                          Программы ещё не загружены
                        </p>
                      ) : (
                        <div className="overflow-x-auto">
                          <table className="w-full">
                            <thead>
                              <tr className="text-left text-sm text-gray-400 border-b border-gray-700">
                                <th className="pb-2 pr-4">#</th>
                                <th className="pb-2 pr-4">Программа</th>
                                <th className="pb-2 pr-4">Команда</th>
                                <th className="pb-2 pr-4 text-center">Очки</th>
                                <th className="pb-2 pr-4 text-center">W</th>
                                <th className="pb-2 pr-4 text-center">L</th>
                                <th className="pb-2 pr-4 text-center">D</th>
                                <th className="pb-2 pr-4 text-center">Игр</th>
                                <th className="pb-2 pr-4">Сборка</th>
                                <th className="pb-2 pr-4">В раундах</th>
                                <th className="pb-2">Действия</th>
                              </tr>
                            </thead>
                            <tbody>
                              {rows.map(({ program, teamName, stats, playing }) => {
                                return (
                                  <tr key={program.id} className="border-b border-gray-800 align-top">
                                    <td className="py-2 pr-4 font-medium text-gray-400">{stats?.rank ?? '–'}</td>
                                    <td className="py-2 pr-4">
                                      <div className="font-medium text-gray-100">
                                        {program.name} <span className="text-xs text-gray-500">v{program.version}</span>
                                      </div>
                                      <code className="text-xs text-gray-500 font-mono">
                                        {program.id.substring(0, 8)}...
                                      </code>
                                    </td>
                                    <td className="py-2 pr-4 text-gray-300">{teamName}</td>
                                    <td className="py-2 pr-4 text-center font-bold text-gray-100">{stats?.rating ?? '–'}</td>
                                    <td className="py-2 pr-4 text-center text-green-400">{stats?.wins ?? '–'}</td>
                                    <td className="py-2 pr-4 text-center text-red-400">{stats?.losses ?? '–'}</td>
                                    <td className="py-2 pr-4 text-center text-gray-400">{stats?.draws ?? '–'}</td>
                                    <td className="py-2 pr-4 text-center text-gray-300">{stats?.total_games ?? '–'}</td>
                                    <td className="py-2 pr-4">
                                      {program.status === 'failed' && program.error_message ? (
                                        <details>
                                          <summary className="w-fit cursor-pointer">
                                            <StatusLabel entity="program" status={program.status} />
                                          </summary>
                                          <pre className="mt-1 max-w-md whitespace-pre-wrap break-words font-mono text-xs text-gray-300">{program.error_message}</pre>
                                        </details>
                                      ) : (
                                        <StatusLabel entity="program" status={program.status} />
                                      )}
                                    </td>
                                    <td className={`py-2 pr-4 text-sm whitespace-nowrap ${playing === 'none' ? 'text-gray-500' : 'text-gray-300'}`}>
                                      {playingLabel[playing]}
                                    </td>
                                    <td className="py-2">
                                      <button
                                        onClick={() => handleDownloadProgram(program)}
                                        className="text-primary-400 hover:text-primary-300 text-sm whitespace-nowrap"
                                        title="Скачать программу"
                                      >
                                        Скачать
                                      </button>
                                    </td>
                                  </tr>
                                );
                              })}
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
