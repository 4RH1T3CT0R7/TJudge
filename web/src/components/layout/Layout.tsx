import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';
import { useDarkMode } from '../../hooks/useDarkMode';

export function Layout() {
  const { user, isAuthenticated, logout } = useAuthStore();
  useDarkMode();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  return (
    <div className="min-h-screen flex flex-col bg-gray-950">
      {/* Glassmorphism Header */}
      <header className="fixed top-0 left-0 right-0 z-50 bg-gray-950/80 backdrop-blur-xl border-b border-gray-800/50">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="relative flex h-16 items-center justify-between">
            {/* Logo with glow */}
            <Link to="/" className="flex items-center shrink-0 z-10">
              <span
                className="text-xl font-bold text-primary-400"
                style={{ textShadow: '0 0 20px rgba(168,85,247,0.4)' }}
              >
                TJudge
              </span>
            </Link>

            {/* Center navigation */}
            <nav className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 flex items-center gap-1">
              <Link
                to="/tournaments"
                className="text-gray-300 hover:text-primary-400 hover:bg-gray-800/70 px-4 py-2 rounded-lg text-sm font-medium transition-colors whitespace-nowrap"
              >
                Турниры
              </Link>
              {isAuthenticated && (
                <Link
                  to="/games"
                  className="text-gray-300 hover:text-primary-400 hover:bg-gray-800/70 px-4 py-2 rounded-lg text-sm font-medium transition-colors whitespace-nowrap"
                >
                  Игры
                </Link>
              )}
              {user?.role === 'admin' && (
                <Link
                  to="/admin"
                  className="text-gray-300 hover:text-primary-400 hover:bg-gray-800/70 px-4 py-2 rounded-lg text-sm font-medium transition-colors whitespace-nowrap"
                >
                  Админ
                </Link>
              )}
            </nav>

            {/* Auth section */}
            <div className="flex items-center gap-3 shrink-0 z-10">
              {isAuthenticated ? (
                <>
                  <Link
                    to="/profile"
                    className="text-sm text-gray-300 hover:text-primary-400 transition-colors"
                  >
                    {user?.username}
                  </Link>
                  <button
                    onClick={handleLogout}
                    className="btn btn-secondary text-sm"
                  >
                    Выйти
                  </button>
                </>
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
      <main className="flex-grow max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 pt-24 w-full">
        <Outlet />
      </main>

      {/* Minimal Footer */}
      <footer className="border-t border-gray-800/50 mt-auto">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
            <p className="text-gray-500 text-sm">
              TJudge — Турнирная система по теории игр
            </p>
            <a
              href="https://itsbmstu.ru"
              target="_blank"
              rel="noopener noreferrer"
              className="group flex gap-3 items-center opacity-70 hover:opacity-100 transition-all duration-300"
            >
              <div className="relative">
                <div className="absolute inset-0 bg-primary-500/50 rounded-lg blur-md opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
                <div className="relative w-10 h-10 bg-gray-800 rounded-lg p-1 group-hover:shadow-[0_0_20px_rgba(168,85,247,0.5)] transition-shadow duration-300">
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
    </div>
  );
}
