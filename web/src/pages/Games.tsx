import { Link } from 'react-router-dom';
import { useGames } from '../hooks/queries';
import { SpaceInvader } from '../components/SpaceInvader';
import { Spinner } from '../components/ui/Spinner';
import { EmptyState } from '../components/ui/EmptyState';
import { ErrorState } from '../components/ui/ErrorState';
import { useDelayedLoading } from '../hooks/useDelayedLoading';
import { getGameConfig } from '../utils/gameConfig';

export function Games() {
  const { data, isPending, isError, refetch } = useGames();
  const showLoading = useDelayedLoading(isPending);
  const games = data ?? [];

  if (showLoading) {
    return <div className="flex justify-center py-24 text-sm text-gray-400"><Spinner>загрузка игр</Spinner></div>;
  }

  if (isPending) {
    return null;
  }

  if (isError) {
    return (
      <div className="pt-12">
        <div className="flex justify-center">
          <SpaceInvader size="sm" controlledPose="dizzy" speechBubble="// ошибка загрузки" eyeOverride="sad" />
        </div>
        <ErrorState message="Не удалось загрузить список игр" onRetry={() => void refetch()} />
      </div>
    );
  }

  return (
    <div>
      <div className="mb-8">
        <title>Игры — TJudge</title>
        <h1 className="text-3xl font-bold text-gray-100">Доступные игры</h1>
        <p className="mt-2 text-gray-400">
          Список игр, в которые можно играть на платформе TJudge
        </p>
      </div>

      {games.length === 0 ? (
        <div className="pt-12">
          <div className="flex justify-center">
            <SpaceInvader size="sm" controlledPose="cry" speechBubble="// пусто..." eyeOverride="sad" />
          </div>
          <EmptyState command="игры" hint="игры пока не добавлены, скоро появятся" />
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
          {games.map((game) => {
            const config = getGameConfig(game.name);
            return (
              <Link
                key={game.id}
                to={`/games/${game.id}`}
                className={`card card-interactive group border-2 ${config.cardBorderClass} transition-[border-color,box-shadow,transform]`}
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
                        .replace(/~~([^~]+)~~/g, '$1')
                        .replace(/\*{1,3}([^*]+)\*{1,3}/g, '$1')
                        .replace(/`([^`]+)`/g, '$1')
                        .replace(/\[([^\]]+)\]\([^)]+\)/g, '$1')
                        .replace(/^[-*+]\s+/gm, '')
                        .replace(/^\d+\.\s+/gm, '')
                        .replace(/^>\s+/gm, '')
                        .replace(/^\|.+\|$/gm, '')
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
