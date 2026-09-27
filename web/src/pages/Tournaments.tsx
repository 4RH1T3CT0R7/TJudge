import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useTournaments } from '../hooks/queries';
import { SpaceInvader } from '../components/SpaceInvader';
import { StaggerList, StaggerItem } from '../components/motion/StaggerList';
import { Spinner } from '../components/ui/Spinner';
import { EmptyState } from '../components/ui/EmptyState';
import { ErrorState } from '../components/ui/ErrorState';
import { useDelayedLoading } from '../hooks/useDelayedLoading';
import { UsersIcon } from '../components/icons';
import { StatusLabel } from '../components/ui/StatusLabel';
import { PageHeader } from '../components/ui/PageHeader';
import type { TournamentStatus } from '../types';

export function Tournaments() {
  const [filter, setFilter] = useState<TournamentStatus | ''>('');
  const { data, isPending, isError, refetch } = useTournaments(filter || undefined);
  const showLoading = useDelayedLoading(isPending);
  const tournaments = data ?? [];

  return (
    <div>
      <PageHeader
        crumbs={[{ label: 'турниры' }]}
        title={
          <>
            <title>Турниры — TJudge</title>
            Турниры
          </>
        }
        actions={
          <select
            aria-label="Статус турниров"
            value={filter}
            onChange={(e) => setFilter(e.target.value as TournamentStatus | '')}
            className="input w-auto min-w-[150px]"
          >
            <option value="">Все статусы</option>
            <option value="pending">Ожидание</option>
            <option value="active">Активные</option>
            <option value="completed">Завершённые</option>
          </select>
        }
      >
        <p className="text-gray-400 text-sm">Найдите турнир и присоединяйтесь к соревнованию</p>
      </PageHeader>

      {/* Content */}
      {showLoading ? (
        <div className="flex justify-center py-24 text-sm text-gray-400"><Spinner>загрузка турниров</Spinner></div>
      ) : isPending ? null : isError && !data ? (
        <div className="pt-12">
          <div className="flex justify-center">
            <SpaceInvader size="sm" controlledPose="dizzy" speechBubble="// ошибка загрузки" eyeOverride="sad" />
          </div>
          <ErrorState message="Не удалось загрузить список турниров" onRetry={() => void refetch()} />
        </div>
      ) : tournaments.length === 0 ? (
        <div className="pt-16">
          <div className="flex justify-center">
            <SpaceInvader size="md" controlledPose="cry" eyeOverride="sad" speechBubble="// пусто..." />
          </div>
          <EmptyState
            command="турниры"
            hint={filter ? 'турниров с таким статусом нет' : 'пока нет доступных турниров: их создаёт организатор в панели админа'}
          />
        </div>
      ) : (
        <StaggerList className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {tournaments.map((tournament) => {
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
                    <StatusLabel entity="tournament" status={tournament.status} />
                  </div>

                  {tournament.description && (
                    <p className="text-gray-400 text-sm mb-3 line-clamp-2">
                      {tournament.description}
                    </p>
                  )}

                  <div className="flex items-center justify-between text-sm text-gray-400">
                    <div className="flex items-center gap-1">
                      <UsersIcon className="w-4 h-4" />
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
