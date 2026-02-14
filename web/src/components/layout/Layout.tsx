import { useState, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';
import { useDarkMode } from '../../hooks/useDarkMode';
import { SpaceInvader } from '../SpaceInvader';
import { AnimatedOutlet } from '../motion/AnimatedOutlet';
import { useKonamiCode, useGodMode } from '../../hooks/useEasterEggs';

export function Layout() {
  const { user, isAuthenticated, logout } = useAuthStore();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  useDarkMode();
  const navigate = useNavigate();

  // Easter eggs
  const { godMode, activateGodMode } = useGodMode();
  useKonamiCode(useCallback(() => {
    activateGodMode();
  }, [activateGodMode]));

  const handleLogout = () => {
    setIsLoggingOut(true);
    // Delay actual logout so dissolve animation plays fully before auth state changes
    setTimeout(async () => {
      await logout();
      navigate('/login');
    }, 1000);
  };

  return (
    <div className="min-h-screen flex flex-col" style={{ backgroundColor: '#0a0a0b' }}>
      {/* Glassmorphism Header — no border */}
      <header className="fixed top-0 left-0 right-0 z-50 backdrop-blur-xl" style={{ backgroundColor: 'rgba(10,10,11,0.8)' }}>
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="relative flex h-16 items-center justify-between">
            {/* Logo with neon glow */}
            <Link
              to="/"
              className="flex items-center gap-2 shrink-0 z-10"
            >
              <span
                className="text-xl font-bold text-primary-400"
                style={{ textShadow: '0 0 20px rgba(139,92,246,0.5)' }}
              >
                TJudge
              </span>
            </Link>

            {/* Center navigation */}
            <nav className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 flex items-center gap-1">
              {isAuthenticated && (
                <>
                  <Link
                    to="/tournaments"
                    className="text-gray-400 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200 whitespace-nowrap hover:text-primary-400"
                    style={{ transitionProperty: 'color, text-shadow' }}
                    onMouseEnter={(e) => { e.currentTarget.style.textShadow = '0 0 12px rgba(139,92,246,0.5)'; }}
                    onMouseLeave={(e) => { e.currentTarget.style.textShadow = 'none'; }}
                  >
                    Турниры
                  </Link>
                  <Link
                    to="/games"
                    className="text-gray-400 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200 whitespace-nowrap hover:text-primary-400"
                    onMouseEnter={(e) => { e.currentTarget.style.textShadow = '0 0 12px rgba(139,92,246,0.5)'; }}
                    onMouseLeave={(e) => { e.currentTarget.style.textShadow = 'none'; }}
                  >
                    Игры
                  </Link>
                </>
              )}
              {user?.role === 'admin' && (
                <Link
                  to="/admin"
                  className="text-gray-400 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200 whitespace-nowrap hover:text-primary-400"
                  onMouseEnter={(e) => { e.currentTarget.style.textShadow = '0 0 12px rgba(139,92,246,0.5)'; }}
                  onMouseLeave={(e) => { e.currentTarget.style.textShadow = 'none'; }}
                >
                  Админ
                </Link>
              )}
            </nav>

            {/* Auth section */}
            <div className="flex items-center gap-3 shrink-0 z-10">
              {isAuthenticated ? (
                <div className={`flex items-center gap-3 ${isLoggingOut ? 'animate-pixel-dissolve' : ''}`}>
                  <Link
                    to="/profile"
                    className="text-sm text-gray-400 hover:text-primary-400 transition-all duration-200"
                    onMouseEnter={(e) => { e.currentTarget.style.textShadow = '0 0 12px rgba(139,92,246,0.4)'; }}
                    onMouseLeave={(e) => { e.currentTarget.style.textShadow = 'none'; }}
                  >
                    {user?.username}
                  </Link>
                  {isLoggingOut && (
                    <SpaceInvader size="sm" controlledPose="cry" eyeOverride="sad" speechBubble="// до свидания..." />
                  )}
                  <button
                    onClick={handleLogout}
                    disabled={isLoggingOut}
                    className="btn btn-secondary text-sm"
                  >
                    {isLoggingOut ? '// ...' : 'Выйти'}
                  </button>
                </div>
              ) : (
                <Link to="/login" className="btn btn-primary text-sm">
                  Войти
                </Link>
              )}
            </div>
          </div>
        </div>
      </header>

      {/* Main content with top padding for fixed header */}
      <main className="flex-grow flex flex-col max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 pt-24 w-full">
        <AnimatedOutlet />
      </main>

      {/* Minimal Footer — no border */}
      <footer className="mt-auto">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
            {/* Partner logos */}
            <div className="flex items-center gap-4">
              {[
                { src: '/bmstu-logo.png', alt: 'МГТУ им. Баумана', h: 'h-9' },
                { src: '/studsovet-logo.png', alt: 'Студсовет МГТУ', h: 'h-8' },
                { src: '/iu-logo.png', alt: 'Студ ИУ', h: 'h-7' },
                { src: '/its-bmstu-logo.png', alt: 'ITS BMSTU', h: 'h-7' },
                { src: '/bcg-logo.png', alt: 'BCG', h: 'h-7' },
              ].map((logo) => (
                <img
                  key={logo.src}
                  src={logo.src}
                  alt={logo.alt}
                  className={`${logo.h} w-auto opacity-50 hover:opacity-90 transition-opacity duration-300`}
                />
              ))}
            </div>
            <a
              href="https://itsbmstu.ru"
              target="_blank"
              rel="noopener noreferrer"
              className="group flex gap-3 items-center opacity-70 hover:opacity-100 transition-all duration-300"
            >
              <div className="relative">
                <div className="absolute inset-0 bg-primary-500/50 rounded-lg blur-md opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
                <div className="relative w-10 h-10 rounded-lg p-1 transition-shadow duration-300" style={{ backgroundColor: 'rgba(31,41,55,0.8)' }}>
                  <img
                    alt="ITS Tech"
                    width="32"
                    height="32"
                    src="/itstech_logo.svg"
                    className="w-full h-full"
                  />
                </div>
              </div>
              <span className="text-base font-medium text-gray-400 group-hover:text-primary-400 transition-colors duration-300">
                Сделано в ИТС ТЕХ
              </span>
            </a>
          </div>
        </div>
      </footer>

      {/* God mode scanline overlay */}
      {godMode && (
        <div className="fixed inset-0 z-[80] pointer-events-none animate-scanline-flash" style={{ mixBlendMode: 'overlay' }}>
          <div className="w-full h-full" style={{
            background: 'repeating-linear-gradient(0deg, transparent, transparent 2px, rgba(139,92,246,0.03) 2px, rgba(139,92,246,0.03) 4px)',
            animation: 'scanline-flash 0.1s ease-out',
          }} />
        </div>
      )}

    </div>
  );
}
