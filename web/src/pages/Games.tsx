import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import api from '../api/client';
import { SpaceInvader } from '../components/SpaceInvader';
import { TerminalLoader } from '../components/TerminalLoader';
import { useDelayedLoading } from '../hooks/useDelayedLoading';
import type { Game } from '../types';

// Game-specific icons and colors configuration (см. https://github.com/bmstu-itstech/tjudge-cli)
const gameConfig: Record<string, { icon: string; color: string; bgClass: string; textClass: string; borderClass: string }> = {
  dilemma: {
    icon: '🤝',
    color: 'purple',
    bgClass: 'bg-primary-500',
    textClass: 'text-primary-400',
    borderClass: 'border-primary-800 hover:border-primary-600',
  },
  tug_of_war: {
    icon: '🪢',
    color: 'green',
    bgClass: 'bg-green-500',
    textClass: 'text-green-400',
    borderClass: 'border-green-800 hover:border-green-600',
  },
  travelers_dilemma: {
    icon: '🧳',
    color: 'blue',
    bgClass: 'bg-blue-500',
    textClass: 'text-blue-400',
    borderClass: 'border-blue-800 hover:border-blue-600',
  },
  public_goods: {
    icon: '🏛️',
    color: 'orange',
    bgClass: 'bg-orange-500',
    textClass: 'text-orange-400',
    borderClass: 'border-orange-800 hover:border-orange-600',
  },
  dollar_auction: {
    icon: '💰',
    color: 'yellow',
    bgClass: 'bg-yellow-500',
    textClass: 'text-yellow-400',
    borderClass: 'border-yellow-800 hover:border-yellow-600',
  },
};

// Default config for unknown games
const defaultGameConfig = {
  icon: '🎮',
  color: 'gray',
  bgClass: 'bg-primary-600',
  textClass: 'text-primary-400',
  borderClass: 'border-gray-700 hover:border-gray-600',
};

const getGameConfig = (gameName: string) => gameConfig[gameName] || defaultGameConfig;

export function Games() {
  const [games, setGames] = useState<Game[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const showLoading = useDelayedLoading(isLoading);
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

  if (showLoading) {
    return <TerminalLoader />;
  }

  if (isLoading) {
    return null;
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <div className="flex justify-center mb-4">
          <SpaceInvader size="sm" controlledPose="dizzy" speechBubble="// ошибка загрузки" eyeOverride="sad" />
        </div>
        <p className="text-red-400">{error}</p>
        <button onClick={loadGames} className="btn btn-secondary mt-4">
          Попробовать снова
        </button>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-100">Доступные игры</h1>
        <p className="mt-2 text-gray-400">
          Список игр, в которые можно играть на платформе TJudge
        </p>
      </div>

      {games.length === 0 ? (
        <div className="text-center py-12">
          <SpaceInvader size="sm" controlledPose="cry" speechBubble="// пусто..." eyeOverride="sad" />
          <p className="text-gray-400 mt-4">Игры пока не добавлены</p>
          <p className="text-gray-500 text-xs mt-1 font-mono">// скоро появятся</p>
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {games.map((game) => {
            const config = getGameConfig(game.name);
            return (
              <Link
                key={game.id}
                to={`/games/${game.id}`}
                className={`card card-interactive group border-2 ${config.borderClass} transition-[border-color,box-shadow,transform]`}
                onMouseEnter={(e) => { e.currentTarget.style.boxShadow = '0 0 30px rgba(139,92,246,0.12), 0 4px 20px rgba(0,0,0,0.3)'; }}
                onMouseLeave={(e) => { e.currentTarget.style.boxShadow = 'none'; }}
              >
                <div className="flex items-start justify-between mb-2">
                  <h2 className="text-xl font-semibold text-gray-100 transition-colors">
                    {game.display_name}
                  </h2>
                  <div className={`w-12 h-12 ${config.bgClass} rounded-xl flex items-center justify-center text-2xl flex-shrink-0 shadow-lg`}>
                    {config.icon}
                  </div>
                </div>
                <p className="text-sm text-gray-400 mb-4">
                  <code className="bg-gray-800 text-gray-100 px-2 py-0.5 rounded font-mono text-sm">{game.name}</code>
                </p>

                {game.rules && (
                  <div className="text-sm text-gray-300 mb-4 line-clamp-3">
                    {(() => {
                      const plain = game.rules
                        .replace(/#{1,6}\s+/g, '')
                        .replace(/\*{1,3}([^*]+)\*{1,3}/g, '$1')
                        .replace(/`([^`]+)`/g, '$1')
                        .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
                        .replace(/^[-*+]\s+/gm, '')
                        .replace(/^\d+\.\s+/gm, '')
                        .replace(/^>\s+/gm, '')
                        .replace(/\n{2,}/g, ' ')
                        .replace(/\n/g, ' ')
                        .trim();
                      return plain.length > 150 ? plain.substring(0, 150) + '...' : plain;
                    })()}
                  </div>
                )}

                <div className={`flex items-center gap-2 ${config.textClass} text-sm font-medium pt-4 border-t border-gray-700`}>
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
