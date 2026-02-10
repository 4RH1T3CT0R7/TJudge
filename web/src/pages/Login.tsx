import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';

export function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const { login, isLoading } = useAuthStore();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    try {
      await login(username, password);
      navigate('/tournaments');
    } catch {
      setError('Неверное имя пользователя или пароль');
    }
  };

  return (
    <div className="min-h-[70vh] flex items-center justify-center">
      <div className="w-full max-w-sm">
        {/* Header */}
        <div className="text-center mb-8">
          <p className="text-sm text-gray-500 mb-2 font-mono">// авторизация</p>
          <h1 className="text-3xl font-bold text-gray-100">Войти в TJudge</h1>
        </div>

        {error && (
          <div className="bg-red-900/20 border border-red-800/50 text-red-400 px-4 py-3 rounded-lg mb-6 text-sm">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-5">
          <div>
            <label htmlFor="username" className="block text-sm text-gray-400 mb-1.5">
              Имя пользователя
            </label>
            <input
              type="text"
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full px-4 py-2.5 bg-gray-900 border border-gray-800 rounded-lg text-gray-100 placeholder:text-gray-600 focus:border-primary-500 focus:ring-1 focus:ring-primary-500/30 outline-none transition-colors"
              autoComplete="username"
              required
              placeholder="username"
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-sm text-gray-400 mb-1.5">
              Пароль
            </label>
            <input
              type="password"
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full px-4 py-2.5 bg-gray-900 border border-gray-800 rounded-lg text-gray-100 placeholder:text-gray-600 focus:border-primary-500 focus:ring-1 focus:ring-primary-500/30 outline-none transition-colors"
              autoComplete="current-password"
              required
              placeholder="••••••••"
            />
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="w-full btn btn-primary py-2.5"
          >
            {isLoading ? 'Вход...' : 'Войти'}
          </button>
        </form>

      </div>
    </div>
  );
}
