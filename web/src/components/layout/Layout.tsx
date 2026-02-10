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
    <div className="min-h-screen flex flex-col" style={{ backgroundColor: '#0a0a0b' }}>
      {/* Glassmorphism Header — no border */}
      <header className="fixed top-0 left-0 right-0 z-50 backdrop-blur-xl" style={{ backgroundColor: 'rgba(10,10,11,0.8)' }}>
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="relative flex h-16 items-center justify-between">
            {/* Logo with neon glow */}
            <Link to="/" className="flex items-center shrink-0 z-10">
              <span
                className="text-xl font-bold text-primary-400"
                style={{ textShadow: '0 0 20px rgba(139,92,246,0.5)' }}
              >
                TJudge
              </span>
            </Link>

            {/* Center navigation */}
            <nav className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 flex items-center gap-1">
              <Link
                to="/tournaments"
                className="text-gray-400 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200 whitespace-nowrap hover:text-primary-400"
                style={{ transitionProperty: 'color, text-shadow' }}
                onMouseEnter={(e) => { e.currentTarget.style.textShadow = '0 0 12px rgba(139,92,246,0.5)'; }}
                onMouseLeave={(e) => { e.currentTarget.style.textShadow = 'none'; }}
              >
                Турниры
              </Link>
              {isAuthenticated && (
                <Link
                  to="/games"
                  className="text-gray-400 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200 whitespace-nowrap hover:text-primary-400"
                  onMouseEnter={(e) => { e.currentTarget.style.textShadow = '0 0 12px rgba(139,92,246,0.5)'; }}
                  onMouseLeave={(e) => { e.currentTarget.style.textShadow = 'none'; }}
                >
                  Игры
                </Link>
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
                <>
                  <Link
                    to="/profile"
                    className="text-sm text-gray-400 hover:text-primary-400 transition-all duration-200"
                    onMouseEnter={(e) => { e.currentTarget.style.textShadow = '0 0 12px rgba(139,92,246,0.4)'; }}
                    onMouseLeave={(e) => { e.currentTarget.style.textShadow = 'none'; }}
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

      {/* Minimal Footer — no border */}
      <footer className="mt-auto">
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
    </div>
  );
}
