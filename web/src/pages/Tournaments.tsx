import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../api/client';
import type { Tournament, TournamentStatus } from '../types';

const statusColors: Record<TournamentStatus, string> = {
  pending: 'bg-yellow-100 text-yellow-800',
  active: 'bg-green-100 text-green-800',
  completed: 'bg-gray-100 text-gray-800',
};

export function Tournaments() {
  const [tournaments, setTournaments] = useState<Tournament[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [filter, setFilter] = useState<TournamentStatus | ''>('');

  useEffect(() => {
    loadTournaments();
  }, [filter]);

  const loadTournaments = async () => {
    setIsLoading(true);
    try {
      const data = await api.getTournaments(filter || undefined);
      setTournaments(data || []);
    } catch (err) {
      console.error('Failed to load tournaments:', err);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Tournaments</h1>

        {/* Filter */}
        <select
          value={filter}
          onChange={(e) => setFilter(e.target.value as TournamentStatus | '')}
          className="input w-auto"
        >
          <option value="">All</option>
          <option value="pending">Pending</option>
          <option value="active">Active</option>
          <option value="completed">Completed</option>
        </select>
      </div>

      {isLoading ? (
        <div className="text-center py-12">
          <p className="text-gray-500">Loading tournaments...</p>
        </div>
      ) : tournaments.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-gray-500">No tournaments found</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {tournaments.map((tournament) => (
            <Link
              key={tournament.id}
              to={`/tournaments/${tournament.id}`}
              className="card hover:shadow-lg transition-shadow"
            >
              <div className="flex justify-between items-start mb-2">
                <h3 className="text-lg font-semibold">{tournament.name}</h3>
                <span className={`px-2 py-1 rounded text-xs font-medium ${statusColors[tournament.status]}`}>
                  {tournament.status}
                </span>
              </div>

              {tournament.description && (
                <p className="text-gray-600 text-sm mb-3 line-clamp-2">
                  {tournament.description}
                </p>
              )}

              <div className="text-sm text-gray-500 space-y-1">
                <p>Code: <code className="bg-gray-100 px-1 rounded">{tournament.code}</code></p>
                <p>Max team size: {tournament.max_team_size}</p>
                {tournament.is_permanent && (
                  <span className="inline-block bg-blue-100 text-blue-800 px-2 py-0.5 rounded text-xs">
                    Permanent
                  </span>
                )}
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
