import { Link, Outlet, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';

export function Layout() {
  const { user, isAuthenticated, logout } = useAuthStore();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white shadow-sm">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16 items-center">
            {/* Logo */}
            <Link to="/" className="flex items-center">
              <span className="text-xl font-bold text-primary-600">TJudge</span>
            </Link>

            {/* Navigation */}
            <nav className="flex items-center space-x-4">
              <Link
                to="/tournaments"
                className="text-gray-700 hover:text-primary-600 px-3 py-2 rounded-md text-sm font-medium"
              >
                Турниры
              </Link>
              {isAuthenticated && (
                <Link
                  to="/games"
                  className="text-gray-700 hover:text-primary-600 px-3 py-2 rounded-md text-sm font-medium"
                >
                  Игры
                </Link>
              )}
              {user?.role === 'admin' && (
                <Link
                  to="/admin"
                  className="text-gray-700 hover:text-primary-600 px-3 py-2 rounded-md text-sm font-medium"
                >
                  Админ
                </Link>
              )}
            </nav>

            {/* Auth section */}
            <div className="flex items-center space-x-4">
              {isAuthenticated ? (
                <>
                  <span className="text-sm text-gray-700">{user?.username}</span>
                  <button
                    onClick={handleLogout}
                    className="btn btn-secondary text-sm"
                  >
                    Выйти
                  </button>
                </>
              ) : (
                <>
                  <Link to="/login" className="btn btn-secondary text-sm">
                    Войти
                  </Link>
                  <Link to="/register" className="btn btn-primary text-sm">
                    Регистрация
                  </Link>
                </>
              )}
            </div>
          </div>
        </div>
      </header>

      {/* Main content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Outlet />
      </main>

      {/* Footer */}
      <footer className="bg-white border-t mt-auto">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <p className="text-center text-gray-500 text-sm">
            TJudge — Турнирная система по теории игр
          </p>
        </div>
      </footer>
    </div>
  );
}
