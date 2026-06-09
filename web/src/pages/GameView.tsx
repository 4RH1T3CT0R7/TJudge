import { useParams, Link } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { useGame } from '../hooks/queries';
import { InvaderPresence } from '../components/motion/InvaderPresence';
import { SpaceInvader } from '../components/SpaceInvader';
import { TerminalLoader } from '../components/TerminalLoader';
import { useDelayedLoading } from '../hooks/useDelayedLoading';

const remarkPlugins = [remarkGfm];

export function GameView() {
  const { id } = useParams<{ id: string }>();
  const { data: game, isPending, isError } = useGame(id ?? '');
  const showLoading = useDelayedLoading(isPending);

  if (showLoading) {
    return <TerminalLoader />;
  }

  if (isPending) {
    return null;
  }

  if (isError || !game) {
    return (
      <div className="text-center py-12">
        <div className="flex justify-center mb-4">
          <SpaceInvader size="sm" controlledPose="cry" speechBubble="// игра не найдена" eyeOverride="sad" />
        </div>
        <p className="text-red-400 mb-4">
          {isError ? 'Не удалось загрузить информацию об игре' : 'Игра не найдена'}
        </p>
        <Link to="/games" className="btn btn-secondary">
          Назад к списку игр
        </Link>
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      {/* Breadcrumb */}
      <nav className="mb-4 text-sm">
        <Link to="/games" className="text-gray-400 hover:text-gray-300">
          Игры
        </Link>
        <span className="mx-2 text-gray-600">/</span>
        <span className="text-gray-200">{game.display_name}</span>
      </nav>

      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center gap-4 mb-2">
          <h1 className="text-3xl font-bold text-gray-100">{game.display_name}</h1>
          <code className="bg-gray-800 text-gray-100 px-3 py-1 rounded font-mono text-sm">{game.name}</code>
        </div>
        <p className="text-gray-400">
          Добавлена {new Date(game.created_at).toLocaleDateString('ru-RU')}
        </p>
      </div>

      {/* Content */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Rules */}
        <div className="lg:col-span-2">
          <div className="card">
            <h2 className="text-xl font-semibold mb-4 text-gray-100">Правила игры</h2>
            {game.rules ? (
              <div className="prose max-w-none prose-invert">
                <div className="markdown-content text-gray-300">
                  <ReactMarkdown remarkPlugins={remarkPlugins}>{game.rules}</ReactMarkdown>
                </div>
              </div>
            ) : (
              <p className="text-gray-400">Правила для этой игры не указаны.</p>
            )}
          </div>
        </div>

        {/* Sidebar */}
        <div className="lg:col-span-1">
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
