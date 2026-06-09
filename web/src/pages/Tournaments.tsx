import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useTournaments } from '../hooks/queries';
import { SpaceInvader } from '../components/SpaceInvader';
import { StaggerList, StaggerItem } from '../components/motion/StaggerList';
import { TerminalLoader } from '../components/TerminalLoader';
import { useDelayedLoading } from '../hooks/useDelayedLoading';
import type { TournamentStatus } from '../types';

const UsersIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
    <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />
  </svg>
);

const statusLabels: Record<TournamentStatus, { label: string; className: string }> = {
  pending: { label: 'Ожидание', className: 'badge badge-yellow' },
  active: { label: 'Активный', className: 'badge badge-green' },
  completed: { label: 'Завершён', className: 'badge badge-gray' },
};

export function Tournaments() {
  const [filter, setFilter] = useState<TournamentStatus | ''>('');
  const { data, isPending } = useTournaments(filter || undefined);
  const showLoading = useDelayedLoading(isPending);
  const tournaments = data ?? [];

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-gray-100 mb-1">Турниры</h1>
            <p className="text-gray-400 text-sm">
              Найдите турнир и присоединяйтесь к соревнованию
            </p>
          </div>

          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value as TournamentStatus | '')}
            className="input w-auto min-w-[150px]"
          >
            <option value="">Все статусы</option>
            <option value="pending">Ожидание</option>
            <option value="active">Активные</option>
            <option value="completed">Завершённые</option>
          </select>
        </div>
      </div>

      {/* Content */}
      {showLoading ? (
        <TerminalLoader />
      ) : isPending ? null : tournaments.length === 0 ? (
        <div className="text-center py-16">
          <div className="relative inline-block">
            <SpaceInvader size="md" controlledPose="cry" eyeOverride="sad" speechBubble="// пусто..." />
          </div>
          <p className="text-gray-400 mt-6 text-xl">
            {filter ? 'Турниры не найдены' : 'Пока нет доступных турниров'}
          </p>
          <p className="text-gray-600 text-base mt-2 font-mono">{'> ожидание новых турниров...'}</p>
          <p className="text-gray-700 text-sm mt-1 font-mono">// создайте первый турнир в панели админа</p>
        </div>
      ) : (
        <StaggerList className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {tournaments.map((tournament) => {
            const status = statusLabels[tournament.status];

            return (
              <StaggerItem key={tournament.id}>
                <Link
                  to={`/tournaments/${tournament.id}`}
                  className="card card-hover block"
                  onMouseEnter={(e) => { e.currentTarget.style.boxShadow = '0 0 30px rgba(139,92,246,0.1), 0 4px 20px rgba(0,0,0,0.3)'; }}
                  onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none'; }}
                >
                  <div className="flex justify-between items-start mb-2">
                    <h3 className="text-base font-semibold text-gray-100 line-clamp-1">
                      {tournament.name}
                    </h3>
                    <span className={status.className}>{status.label}</span>
                  </div>

                  {tournament.description && (
                    <p className="text-gray-400 text-sm mb-3 line-clamp-2">
                      {tournament.description}
                    </p>
                  )}

                  <div className="flex items-center justify-between text-sm text-gray-400">
                    <div className="flex items-center gap-1">
                      <UsersIcon />
                      <span>До {tournament.max_team_size} чел.</span>
                    </div>
                    <code className="bg-gray-700 px-2 py-0.5 rounded text-xs text-gray-300">
                      {tournament.code}
                    </code>
                  </div>

                  {tournament.is_permanent && (
                    <span className="badge badge-blue mt-3 text-xs">Постоянный</span>
                  )}
                </Link>
              </StaggerItem>
            );
          })}
        </StaggerList>
      )}
    </div>
  );
}
