import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';
import { useDarkMode } from '../../hooks/useDarkMode';

// Animated theme toggle component like iOS switch
function ThemeToggle({ isDark, onToggle }: { isDark: boolean; onToggle: () => void }) {
  return (
    <button
      onClick={onToggle}
      className="relative w-14 h-7 rounded-full p-0.5 transition-colors duration-300 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900"
      style={{
        background: isDark
          ? 'linear-gradient(135deg, #1e3a5f 0%, #0f172a 100%)'
          : 'linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%)',
      }}
      aria-label={isDark ? 'Включить светлую тему' : 'Включить тёмную тему'}
    >
      {/* Background stars for dark mode */}
      <div className={`absolute inset-0 overflow-hidden rounded-full transition-opacity duration-300 ${isDark ? 'opacity-100' : 'opacity-0'}`}>
        <div className="absolute top-1.5 left-2 w-0.5 h-0.5 bg-white rounded-full animate-pulse" />
        <div className="absolute top-3 left-4 w-1 h-1 bg-white/60 rounded-full" />
        <div className="absolute bottom-2 left-3 w-0.5 h-0.5 bg-white/80 rounded-full" />
      </div>

      {/* Slider knob */}
      <div
        className={`relative w-6 h-6 rounded-full shadow-lg transform transition-all duration-300 ease-spring ${
          isDark ? 'translate-x-7' : 'translate-x-0'
        }`}
        style={{
          background: isDark
            ? 'linear-gradient(135deg, #e2e8f0 0%, #94a3b8 100%)'
            : 'linear-gradient(135deg, #fef3c7 0%, #fcd34d 100%)',
          boxShadow: isDark
            ? '0 2px 8px rgba(0, 0, 0, 0.4), inset 0 -1px 2px rgba(0, 0, 0, 0.1)'
            : '0 2px 8px rgba(245, 158, 11, 0.4), inset 0 -1px 2px rgba(0, 0, 0, 0.1)',
        }}
      >
        {/* Sun icon */}
        <div className={`absolute inset-0 flex items-center justify-center transition-all duration-300 ${isDark ? 'opacity-0 rotate-90 scale-50' : 'opacity-100 rotate-0 scale-100'}`}>
          <svg className="w-4 h-4 text-amber-600" fill="currentColor" viewBox="0 0 20 20">
            <path fillRule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clipRule="evenodd" />
          </svg>
        </div>

        {/* Moon icon */}
        <div className={`absolute inset-0 flex items-center justify-center transition-all duration-300 ${isDark ? 'opacity-100 rotate-0 scale-100' : 'opacity-0 -rotate-90 scale-50'}`}>
          <svg className="w-4 h-4 text-slate-600" fill="currentColor" viewBox="0 0 20 20">
            <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
          </svg>
        </div>
      </div>
    </button>
  );
}

export function Layout() {
  const { user, isAuthenticated, logout } = useAuthStore();
  const { isDark, toggle } = useDarkMode();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  return (
    <div className="min-h-screen flex flex-col bg-gray-50 dark:bg-gray-950 transition-colors duration-200">
      {/* Header */}
      <header className="bg-white dark:bg-gray-900 shadow-sm dark:shadow-black/30 transition-colors duration-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="relative flex h-16 items-center justify-between">
            {/* Logo */}
            <Link to="/" className="flex items-center shrink-0 z-10">
              <span className="text-xl font-bold text-primary-600 dark:text-primary-400">TJudge</span>
            </Link>

            {/* Navigation - absolutely centered */}
            <nav className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 flex items-center gap-1">
              <Link
                to="/tournaments"
                className="text-gray-700 dark:text-gray-100 hover:text-primary-600 dark:hover:text-primary-400 hover:bg-gray-100 dark:hover:bg-gray-800 px-4 py-2 rounded-lg text-sm font-medium transition-colors whitespace-nowrap"
              >
                Турниры
              </Link>
              {isAuthenticated && (
                <Link
                  to="/games"
                  className="text-gray-700 dark:text-gray-100 hover:text-primary-600 dark:hover:text-primary-400 hover:bg-gray-100 dark:hover:bg-gray-800 px-4 py-2 rounded-lg text-sm font-medium transition-colors whitespace-nowrap"
                >
                  Игры
                </Link>
              )}
              {user?.role === 'admin' && (
                <Link
                  to="/admin"
                  className="text-gray-700 dark:text-gray-100 hover:text-primary-600 dark:hover:text-primary-400 hover:bg-gray-100 dark:hover:bg-gray-800 px-4 py-2 rounded-lg text-sm font-medium transition-colors whitespace-nowrap"
                >
                  Админ
                </Link>
              )}
            </nav>

            {/* Auth section */}
            <div className="flex items-center gap-3 shrink-0 z-10">
              {/* Dark mode toggle */}
              <ThemeToggle isDark={isDark} onToggle={toggle} />

              {isAuthenticated ? (
                <>
                  <Link
                    to="/profile"
                    className="text-sm text-gray-700 dark:text-gray-100 hover:text-primary-600 dark:hover:text-primary-400 transition-colors"
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

      {/* Main content */}
      <main className="flex-grow max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full">
        <Outlet />
      </main>

      {/* Footer */}
      <footer className="bg-white dark:bg-gray-900 border-t dark:border-gray-800 mt-auto transition-colors duration-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
            <p className="text-gray-500 dark:text-gray-400 text-sm">
              TJudge — Турнирная система по теории игр
            </p>
            <a
              href="https://itsbmstu.ru"
              target="_blank"
              rel="noopener noreferrer"
              className="group flex gap-3 items-center opacity-70 hover:opacity-100 transition-all duration-300"
            >
              <div className="relative">
                <div className="absolute inset-0 bg-blue-500/50 rounded-lg blur-md opacity-0 group-hover:opacity-100 transition-opacity duration-300" />
                <div className="relative w-10 h-10 bg-gray-900 dark:bg-gray-800 rounded-lg p-1 group-hover:shadow-[0_0_20px_rgba(59,130,246,0.5)] transition-shadow duration-300">
                  <img
                    alt="ITS Tech"
                    width="32"
                    height="32"
                    src="/itstech_logo.svg"
                    className="w-full h-full"
                  />
                </div>
              </div>
              <span className="text-base font-medium text-gray-600 dark:text-gray-300 group-hover:text-blue-500 dark:group-hover:text-blue-400 transition-colors duration-300">
                Сделано в ИТС ТЕХ
              </span>
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
