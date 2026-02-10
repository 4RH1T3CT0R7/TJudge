import { Link } from 'react-router-dom';
import { SpaceInvader } from '../components/SpaceInvader';

export function NotFound() {
  return (
    <div className="flex flex-col items-center justify-center min-h-[70vh] py-12 px-4 relative overflow-hidden">
      {/* Glow orbs */}
      <div
        className="absolute top-1/4 -left-20 w-72 h-72 rounded-full opacity-15 blur-3xl pointer-events-none"
        style={{ background: 'radial-gradient(circle, rgba(139,92,246,0.5), transparent 70%)' }}
      />
      <div
        className="absolute bottom-1/4 -right-20 w-72 h-72 rounded-full opacity-10 blur-3xl pointer-events-none"
        style={{ background: 'radial-gradient(circle, rgba(74,222,128,0.4), transparent 70%)' }}
      />

      <div style={{ zIndex: 60, position: 'relative' }}>
        <SpaceInvader size="lg" className="mb-8" interactive />
      </div>

      <h1
        className="text-7xl md:text-9xl font-extrabold mb-4"
        style={{
          background: 'linear-gradient(135deg, #c4b5fd, #8b5cf6, #6d28d9, #c4b5fd)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          backgroundClip: 'text',
        }}
      >
        404
      </h1>

      <p className="text-xl text-gray-400 mb-2">
        Страница не найдена
      </p>
      <p className="text-sm text-gray-500 mb-8">
        Этот инвейдер тоже потерялся в космосе
      </p>

      <Link to="/" className="btn btn-primary">
        Вернуться на главную
      </Link>
    </div>
  );
}
