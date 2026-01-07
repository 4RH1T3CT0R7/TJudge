import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import api from '../api/client';
import type { Game } from '../types';

// Simple Markdown renderer
function MarkdownRenderer({ content }: { content: string }) {
  const parseMarkdown = (text: string): string => {
    let html = text
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      // Headers
      .replace(/^### (.*$)/gim, '<h3 class="text-lg font-semibold mt-4 mb-2">$1</h3>')
      .replace(/^## (.*$)/gim, '<h2 class="text-xl font-semibold mt-6 mb-3">$1</h2>')
      .replace(/^# (.*$)/gim, '<h1 class="text-2xl font-bold mt-6 mb-4">$1</h1>')
      // Bold
      .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
      // Italic
      .replace(/\*(.*?)\*/g, '<em>$1</em>')
      // Code blocks
      .replace(/```(\w*)\n([\s\S]*?)```/g, '<pre class="bg-gray-100 p-3 rounded-lg overflow-x-auto my-3"><code>$2</code></pre>')
      // Inline code
      .replace(/`([^`]+)`/g, '<code class="bg-gray-100 px-1 py-0.5 rounded text-sm">$1</code>')
      // Tables
      .replace(/\|(.+)\|/g, (match) => {
        const cells = match.split('|').filter(c => c.trim());
        if (cells.every(c => c.trim().match(/^-+$/))) {
          return ''; // Skip separator row
        }
        const cellHtml = cells.map(c => `<td class="border border-gray-300 px-3 py-2">${c.trim()}</td>`).join('');
        return `<tr>${cellHtml}</tr>`;
      })
      // Links
      .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" class="text-primary-600 hover:underline" target="_blank" rel="noopener noreferrer">$1</a>')
      // Unordered lists
      .replace(/^\s*[-*] (.*$)/gim, '<li class="ml-4">$1</li>')
      // Paragraphs
      .replace(/\n\n/g, '</p><p class="my-3">')
      .replace(/\n/g, '<br />');

    html = '<p class="my-3">' + html + '</p>';
    return html;
  };

  return (
    <div
      className="markdown-content"
      dangerouslySetInnerHTML={{ __html: parseMarkdown(content) }}
    />
  );
}

export function GameView() {
  const { id } = useParams<{ id: string }>();
  const [game, setGame] = useState<Game | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (id) {
      loadGame();
    }
  }, [id]);

  const loadGame = async () => {
    if (!id) return;

    setIsLoading(true);
    setError(null);

    try {
      const gameData = await api.getGame(id);
      setGame(gameData);
    } catch (err) {
      setError('Не удалось загрузить информацию об игре');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div className="text-center py-12">
        <div className="w-10 h-10 border-4 border-primary-200 border-t-primary-600 rounded-full animate-spin mx-auto mb-4" />
        <p className="text-gray-500">Загрузка игры...</p>
      </div>
    );
  }

  if (error || !game) {
    return (
      <div className="text-center py-12">
        <p className="text-red-500 mb-4">{error || 'Игра не найдена'}</p>
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
        <Link to="/games" className="text-gray-500 hover:text-gray-700">
          Игры
        </Link>
        <span className="mx-2 text-gray-400">/</span>
        <span className="text-gray-900">{game.display_name}</span>
      </nav>

      {/* Header */}
      <div className="mb-8">
        <div className="flex items-center gap-4 mb-2">
          <h1 className="text-3xl font-bold text-gray-900">{game.display_name}</h1>
          <code className="bg-gray-100 px-3 py-1 rounded text-sm text-gray-600">{game.name}</code>
        </div>
        <p className="text-gray-500">
          Добавлена {new Date(game.created_at).toLocaleDateString('ru-RU')}
        </p>
      </div>

      {/* Content */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Rules */}
        <div className="lg:col-span-2">
          <div className="card">
            <h2 className="text-xl font-semibold mb-4">Правила игры</h2>
            {game.rules ? (
              <div className="prose max-w-none">
                <MarkdownRenderer content={game.rules} />
              </div>
            ) : (
              <p className="text-gray-500">Правила для этой игры не указаны.</p>
            )}
          </div>
        </div>

        {/* Sidebar */}
        <div className="lg:col-span-1">
          <div className="card">
            <h2 className="text-lg font-semibold mb-4">Участие</h2>
            <p className="text-gray-600 mb-4">
              Чтобы участвовать в соревнованиях по этой игре, присоединитесь к турниру.
            </p>
            <Link
              to={`/tournaments`}
              className="btn btn-primary w-full"
            >
              Найти турниры
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
