import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQueries } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import { SpaceInvader } from '../SpaceInvader';
import type { Team, TournamentStatus } from '../../types';

// Teams Tab Component
export function TeamsTab({
  teams,
  isAuthenticated,
  isAdmin,
  myTeam,
  tournamentStatus,
  onJoinByCode,
  joinCode,
  setJoinCode,
  isJoining,
  joinError,
  setJoinError,
  onDisqualify,
  onRestore,
}: {
  teams: Team[];
  isAuthenticated: boolean;
  isAdmin: boolean;
  myTeam: Team | null;
  tournamentStatus: TournamentStatus;
  onJoinByCode: () => void;
  joinCode: string;
  setJoinCode: (code: string) => void;
  isJoining: boolean;
  joinError: string;
  setJoinError: (e: string) => void;
  onDisqualify?: (teamId: string) => void;
  onRestore?: (teamId: string) => void;
}) {
  const showJoinSection = isAuthenticated && !myTeam && tournamentStatus === 'pending';
  const [membersExpanded, setMembersExpanded] = useState(false);

  // Составы команд: запросы выполняются только после раскрытия (enabled),
  // кэшируются по queryKeys.team и дедуплицируются между перерисовками.
  const memberQueries = useQueries({
    queries: teams.map((t) => ({
      queryKey: queryKeys.team(t.id),
      queryFn: () => api.getTeam(t.id),
      enabled: membersExpanded,
    })),
  });
  const loadingMembers = membersExpanded && memberQueries.some(q => q.isLoading);
  const teamMembers: Record<string, { username: string; email: string }[]> = {};
  teams.forEach((t, i) => {
    const data = memberQueries[i]?.data;
    if (data) {
      teamMembers[t.id] = data.members.map(m => ({ username: m.username, email: m.email }));
    }
  });

  const toggleAllMembers = () => {
    setMembersExpanded(prev => !prev);
  };

  return (
    <div>
      {/* Join by code section */}
      {showJoinSection && (
        <div className="card mb-6 bg-blue-900/30 border-blue-700">
          <h3 className="font-semibold mb-3 text-blue-200">Присоединиться к команде</h3>
          <p className="text-sm text-blue-300 mb-3">
            Введите код приглашения, полученный от капитана команды
          </p>
          <div className="flex gap-2">
            <input
              type="text"
              value={joinCode}
              onChange={(e) => { setJoinCode(e.target.value.toUpperCase()); setJoinError(''); }}
              placeholder="Код приглашения (например: ABC123)"
              className="input flex-1 uppercase tracking-wider"
              maxLength={10}
            />
            <button
              onClick={onJoinByCode}
              disabled={isJoining || !joinCode.trim()}
              className="btn btn-primary"
            >
              {isJoining ? 'Вступление...' : 'Вступить'}
            </button>
          </div>
          {joinError && <p className="text-red-400 text-sm mt-2">{joinError}</p>}
        </div>
      )}

      {/* Teams list */}
      {teams.length === 0 ? (
        <div className="empty-state">
          <div className="flex justify-center mb-4">
            <SpaceInvader size="sm" controlledPose="cry" speechBubble="// пока никого..." eyeOverride="sad" />
          </div>
          <h3 className="empty-state-title">Нет команд</h3>
          <p className="empty-state-description">
            Ни одна команда еще не присоединилась к турниру
          </p>
        </div>
      ) : (
        <div>
          {isAdmin && (
            <button
              onClick={toggleAllMembers}
              disabled={loadingMembers}
              className="mb-4 inline-flex items-center gap-1.5 text-sm text-gray-400 hover:text-gray-200 transition-colors"
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className={`w-4 h-4 transition-transform ${membersExpanded ? 'rotate-180' : ''}`}>
                <path strokeLinecap="round" strokeLinejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
              </svg>
              {loadingMembers ? 'Загрузка...' : membersExpanded ? 'Скрыть составы' : 'Показать составы команд'}
            </button>
          )}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {teams.map((team, index) => (
              <div
                key={team.id}
                className={`card group hover:shadow-lg hover:shadow-gray-900/50 transition-shadow ${
                  myTeam?.id === team.id
                    ? 'border-2 border-primary-500 bg-primary-900/20'
                    : team.is_disqualified
                    ? 'border border-red-800/50 opacity-60'
                    : ''
                }`}
              >
                <div className="flex items-center gap-3">
                  <div className={`w-10 h-10 rounded-lg flex items-center justify-center text-white font-bold ${
                    team.is_disqualified ? 'bg-red-700' : myTeam?.id === team.id ? 'bg-primary-500' : 'bg-gray-500'
                  }`}>
                    {index + 1}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="font-semibold text-gray-100 truncate">{team.name}</h3>
                      {team.is_disqualified && (
                        <span className="px-2 py-0.5 bg-red-900/50 text-red-400 text-xs rounded-full">Дисквалификация</span>
                      )}
                      {myTeam?.id === team.id && (
                        <span className="badge badge-blue text-xs">Ваша</span>
                      )}
                    </div>
                    <p className="text-sm text-gray-400">
                      {new Date(team.created_at).toLocaleDateString('ru-RU')}
                    </p>
                  </div>
                </div>

                {/* Admin: team members */}
                {isAdmin && membersExpanded && (
                  <div className="mt-3 pt-3 border-t border-gray-700">
                    {loadingMembers ? (
                      <p className="text-xs text-gray-500">Загрузка...</p>
                    ) : (teamMembers[team.id] || []).length === 0 ? (
                      <p className="text-xs text-gray-500">Нет участников</p>
                    ) : (
                      <div className="space-y-1">
                        {teamMembers[team.id].map((member, i) => (
                          <div key={i} className="flex items-center gap-2 text-sm">
                            <span className="w-5 h-5 rounded-full bg-gray-600 flex items-center justify-center text-xs text-gray-300">
                              {member.username[0]?.toUpperCase()}
                            </span>
                            <span className="text-gray-200">{member.username}</span>
                            <span className="text-gray-500 text-xs truncate">{member.email}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {isAdmin && tournamentStatus === 'active' && (
                  <div className="mt-3 pt-3 border-t border-gray-700">
                    {team.is_disqualified ? (
                      <button
                        onClick={() => onRestore?.(team.id)}
                        className="px-3 py-1.5 bg-green-700 hover:bg-green-600 text-white text-xs rounded-lg transition-colors"
                      >
                        Восстановить
                      </button>
                    ) : (
                      <button
                        onClick={() => onDisqualify?.(team.id)}
                        className="px-3 py-1.5 bg-red-700 hover:bg-red-600 text-white text-xs rounded-lg transition-colors"
                      >
                        Дисквалифицировать
                      </button>
                    )}
                  </div>
                )}

                {myTeam?.id === team.id && (
                  <Link
                    to={`/teams/${team.id}`}
                    className="mt-3 inline-flex items-center gap-1 text-primary-400 hover:text-primary-300 text-sm font-medium"
                  >
                    Управление командой
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-4 h-4">
                      <path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
                    </svg>
                  </Link>
                )}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
