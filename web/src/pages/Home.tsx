import { Link } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';

export function Home() {
  const { isAuthenticated } = useAuthStore();

  return (
    <div className="text-center py-12">
      <h1 className="text-4xl font-bold text-gray-900 mb-4">
        Добро пожаловать в TJudge
      </h1>
      <p className="text-xl text-gray-600 mb-8 max-w-2xl mx-auto">
        Турнирная система для соревнований по теории игр.
        Создавайте команды, загружайте программы и участвуйте в турнирах.
      </p>

      <div className="flex justify-center space-x-4">
        <Link to="/tournaments" className="btn btn-primary">
          Смотреть турниры
        </Link>
        {!isAuthenticated && (
          <Link to="/register" className="btn btn-secondary">
            Начать
          </Link>
        )}
      </div>

      {/* Features section */}
      <div className="mt-16 grid grid-cols-1 md:grid-cols-3 gap-8">
        <div className="card">
          <h3 className="text-lg font-semibold mb-2">Командные соревнования</h3>
          <p className="text-gray-600">
            Создавайте команды или присоединяйтесь по коду приглашения.
            Разрабатывайте стратегии вместе с товарищами.
          </p>
        </div>

        <div className="card">
          <h3 className="text-lg font-semibold mb-2">Таблица лидеров в реальном времени</h3>
          <p className="text-gray-600">
            Следите за обновлением рейтинга в реальном времени по мере завершения матчей.
            Поддержка полноэкранного режима.
          </p>
        </div>

        <div className="card">
          <h3 className="text-lg font-semibold mb-2">Несколько игр</h3>
          <p className="text-gray-600">
            Турниры могут включать несколько игр.
            Правила отображаются в формате Markdown.
          </p>
        </div>
      </div>
    </div>
  );
}
