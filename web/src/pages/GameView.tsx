import { useParams, Link } from 'react-router-dom';
import { useGame } from '../hooks/queries';
import { InvaderPresence } from '../components/motion/InvaderPresence';
import { SpaceInvader } from '../components/SpaceInvader';
import { Spinner } from '../components/ui/Spinner';
import { ErrorState } from '../components/ui/ErrorState';
import { PageHeader } from '../components/ui/PageHeader';
import { useDelayedLoading } from '../hooks/useDelayedLoading';
import { GameRules } from '../components/game/GameRules';
import { StarterKit } from '../components/game/StarterKit';
import { RuntimeInfo } from '../components/game/RuntimeInfo';
import { revealAndFocus } from '../hooks/useRevealOnMobile';
import { LANGUAGES, starterLoader } from '../utils/starters';

export function GameView() {
  const { id } = useParams<{ id: string }>();
  const { data: game, isPending, isError, refetch } = useGame(id ?? '');
  const showLoading = useDelayedLoading(isPending);

  if (showLoading) {
    return <div className="flex justify-center py-24 text-sm text-gray-400"><Spinner>загрузка игры</Spinner></div>;
  }

  if (isPending) {
    return null;
  }

  if (isError || !game) {
    return (
      <div className="pt-12">
        <div className="flex justify-center">
          <SpaceInvader size="sm" controlledPose="cry" speechBubble="// игра не найдена" eyeOverride="sad" />
        </div>
        <ErrorState
          message={isError ? 'Не удалось загрузить информацию об игре' : 'Игра не найдена'}
          onRetry={isError ? () => void refetch() : undefined}
        >
          <Link to="/games" className="btn btn-secondary">
            Назад к списку игр
          </Link>
        </ErrorState>
      </div>
    );
  }

  return (
    <div>
      <PageHeader
        crumbs={[{ label: 'игры', to: '/games' }, { label: game.display_name }]}
        title={
          <>
            <title>{`${game.display_name} — TJudge`}</title>
            {game.display_name}
          </>
        }
        status={<code className="bg-gray-800 text-gray-100 px-3 py-1 rounded font-mono text-sm">{game.name}</code>}
        actions={
          // шаблон стоит под правилами, с первого экрана его не видно
          LANGUAGES.some((l) => starterLoader(game.name, l)) && (
            <button
              type="button"
              onClick={() => revealAndFocus(document.getElementById('starter'))}
              className="btn btn-secondary font-mono"
            >
              <span aria-hidden="true">$ </span>начать с шаблона<span aria-hidden="true"> ↓</span>
            </button>
          )
        }
      >
        <p className="text-gray-400">
          Добавлена {new Date(game.created_at).toLocaleDateString('ru-RU')}
        </p>
      </PageHeader>

      {/* Content */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Rules */}
        <div className="lg:col-span-2 min-w-0">
          <div className="card">
            <h2 className="text-xl font-semibold mb-4 text-gray-100">Правила игры</h2>
            {game.rules ? (
              <GameRules rules={game.rules} />
            ) : (
              <p className="text-gray-400">Правила для этой игры не указаны.</p>
            )}
          </div>
          <div className="mt-6">
            <StarterKit game={game.name} />
          </div>
        </div>

        {/* Sidebar */}
        <div className="lg:col-span-1 min-w-0">
          <div className="card">
            <h2 className="text-lg font-semibold mb-4 text-gray-100">Участие</h2>
            <p className="text-gray-400 mb-4">
              Чтобы участвовать в соревнованиях по этой игре, присоединитесь к турниру.
            </p>
            <Link
              to={`/tournaments`}
              className="btn btn-primary w-full"
            >
              Найти турниры
            </Link>
            <div className="mt-4">
              <RuntimeInfo />
            </div>
          </div>
          {/* Invader - вне карточки, чтобы избежать overflow */}
          <div className="flex justify-end mt-3 pr-2">
            <InvaderPresence
              size="sm"
              entrance="slideLeft"
              speechBubble="// попробуй!"
            />
          </div>
        </div>
      </div>
    </div>
  );
}
