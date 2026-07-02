import { CalendarIcon, ClockIcon, UsersIcon } from '../icons';
import type { Tournament } from '../../types';

// Info Tab Component
export function InfoTab({ tournament }: { tournament: Tournament }) {
  return (
    <div className="card">
      {tournament.description ? (
        <div className="prose max-w-none mb-8">
          <p className="text-gray-300 leading-relaxed">{tournament.description}</p>
        </div>
      ) : (
        <p className="text-gray-400 mb-8">Описание не указано.</p>
      )}

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="stat-card">
          <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
            <UsersIcon />
            <span>Макс. размер команды</span>
          </div>
          <p className="text-2xl font-bold text-gray-100">{tournament.max_team_size}</p>
        </div>

        {tournament.max_participants && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <UsersIcon />
              <span>Макс. участников</span>
            </div>
            <p className="text-2xl font-bold text-gray-100">{tournament.max_participants}</p>
          </div>
        )}

        {tournament.start_time && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <CalendarIcon />
              <span>Начало</span>
            </div>
            <p className="text-lg font-bold text-gray-100">
              {new Date(tournament.start_time).toLocaleDateString('ru-RU')}
            </p>
          </div>
        )}

        {tournament.end_time && (
          <div className="stat-card">
            <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
              <CalendarIcon />
              <span>Окончание</span>
            </div>
            <p className="text-lg font-bold text-gray-100">
              {new Date(tournament.end_time).toLocaleDateString('ru-RU')}
            </p>
          </div>
        )}

        <div className="stat-card">
          <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
            <ClockIcon className="w-4 h-4" />
            <span>Создан</span>
          </div>
          <p className="text-lg font-bold text-gray-100">
            {new Date(tournament.created_at).toLocaleDateString('ru-RU')}
          </p>
        </div>
      </div>
    </div>
  );
}
