import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../api/client';
import type { Game } from '../types';

// Game-specific icons and colors configuration
const gameConfig: Record<string, { icon: string; color: string; bgClass: string; textClass: string; borderClass: string }> = {
  prisoners_dilemma: {
    icon: '🤝',
    color: 'blue',
    bgClass: 'bg-blue-500',
    textClass: 'text-blue-600 dark:text-blue-400',
    borderClass: 'border-blue-200 dark:border-blue-800 hover:border-blue-300 dark:hover:border-blue-700',
  },
  tug_of_war: {
    icon: '🪢',
    color: 'emerald',
    bgClass: 'bg-emerald-500',
    textClass: 'text-emerald-600 dark:text-emerald-400',
    borderClass: 'border-emerald-200 dark:border-emerald-800 hover:border-emerald-300 dark:hover:border-emerald-700',
  },
  good_deal: {
    icon: '💰',
    color: 'purple',
    bgClass: 'bg-purple-500',
    textClass: 'text-purple-600 dark:text-purple-400',
    borderClass: 'border-purple-200 dark:border-purple-800 hover:border-purple-300 dark:hover:border-purple-700',
  },
  balance_of_universe: {
    icon: '⚖️',
    color: 'indigo',
    bgClass: 'bg-indigo-500',
    textClass: 'text-indigo-600 dark:text-indigo-400',
    borderClass: 'border-indigo-200 dark:border-indigo-800 hover:border-indigo-300 dark:hover:border-indigo-700',
  },
};

// Default config for unknown games
const defaultGameConfig = {
  icon: '🎮',
  color: 'gray',
  bgClass: 'bg-primary-600',
  textClass: 'text-primary-600 dark:text-primary-400',
  borderClass: 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600',
};

const getGameConfig = (gameName: string) => gameConfig[gameName] || defaultGameConfig;

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
        <p className="text-gray-500 dark:text-gray-400">Загрузка игр...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500 dark:text-red-400">{error}</p>
        <button onClick={loadGames} className="btn btn-secondary mt-4">
          Попробовать снова
        </button>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900 dark:text-gray-100">Доступные игры</h1>
        <p className="mt-2 text-gray-600 dark:text-gray-400">
          Список игр, в которые можно играть на платформе TJudge
        </p>
      </div>

      {games.length === 0 ? (
        <div className="text-center py-12 bg-gray-50 dark:bg-gray-800 rounded-lg">
          <p className="text-gray-500 dark:text-gray-400">Игры пока не добавлены</p>
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {games.map((game) => {
            const config = getGameConfig(game.name);
            return (
              <Link
                key={game.id}
                to={`/games/${game.id}`}
                className={`card card-interactive group border-2 ${config.borderClass} transition-all`}
              >
                <div className="flex items-start justify-between mb-2">
                  <h2 className={`text-xl font-semibold text-gray-900 dark:text-gray-100 group-hover:${config.textClass.split(' ')[0]} transition-colors`}>
                    {game.display_name}
                  </h2>
                  <div className={`w-12 h-12 ${config.bgClass} rounded-xl flex items-center justify-center text-2xl flex-shrink-0 shadow-lg`}>
                    {config.icon}
                  </div>
                </div>
                <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
                  <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{game.name}</code>
                </p>

                {game.rules && (
                  <div className="text-sm text-gray-600 dark:text-gray-300 mb-4 line-clamp-3">
                    {game.rules.substring(0, 150)}
                    {game.rules.length > 150 && '...'}
                  </div>
                )}

                <div className={`flex items-center gap-2 ${config.textClass} text-sm font-medium pt-4 border-t border-gray-100 dark:border-gray-700`}>
                  <span>Подробнее</span>
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" className="w-4 h-4 group-hover:translate-x-1 transition-transform">
                    <path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
                  </svg>
                </div>
              </Link>
            );
          })}
        </div>
      )}
    </div>
  );
}
