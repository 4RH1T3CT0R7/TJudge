import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../api/client';
import type { Game } from '../types';

export function Games() {
  const [games, setGames] = useState<Game[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadGames();
  }, []);

  const loadGames = async () => {
    setIsLoading(true);
    setError(null);

    try {
      const data = await api.getGames();
      setGames(data);
    } catch (err) {
      setError('Не удалось загрузить список игр');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <p className="text-gray-500">Загрузка игр...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500">{error}</p>
        <button onClick={loadGames} className="btn btn-secondary mt-4">
          Попробовать снова
        </button>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">Доступные игры</h1>
        <p className="mt-2 text-gray-600">
          Список игр, в которые можно играть на платформе TJudge
        </p>
      </div>

      {games.length === 0 ? (
        <div className="text-center py-12 bg-gray-50 rounded-lg">
          <p className="text-gray-500">Игры пока не добавлены</p>
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {games.map((game) => (
            <div key={game.id} className="card hover:shadow-lg transition-shadow">
              <h2 className="text-xl font-semibold mb-2">{game.display_name}</h2>
              <p className="text-sm text-gray-500 mb-4">
                Системное имя: <code className="bg-gray-100 px-1 rounded">{game.name}</code>
              </p>

              {game.rules && (
                <div className="text-sm text-gray-600 mb-4 line-clamp-3">
                  {game.rules.substring(0, 150)}
                  {game.rules.length > 150 && '...'}
                </div>
              )}

              <div className="flex justify-between items-center mt-auto pt-4 border-t">
                <span className="text-xs text-gray-400">
                  Добавлена: {new Date(game.created_at).toLocaleDateString('ru-RU')}
                </span>
                <Link
                  to={`/tournaments?game=${game.name}`}
                  className="text-primary-600 hover:text-primary-800 text-sm font-medium"
                >
                  Найти турниры →
                </Link>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
