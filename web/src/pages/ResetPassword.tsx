import { useState } from 'react';
import { Link, useSearchParams, useNavigate } from 'react-router-dom';
import axios from 'axios';

// P1.11: страница завершения восстановления пароля.
// Принимает token из URL query ?token=..., просит новый пароль, вызывает confirm.

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

export function ResetPassword() {
  const [params] = useSearchParams();
  const token = params.get('token') || '';
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [loading, setLoading] = useState(false);
  const [done, setDone] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!token) {
      setError('Токен отсутствует. Используйте ссылку из email.');
      return;
    }
    if (password.length < 8) {
      setError('Пароль должен быть не короче 8 символов.');
      return;
    }
    if (password !== confirm) {
      setError('Пароли не совпадают.');
      return;
    }

    setLoading(true);
    try {
      await axios.post(`${API_BASE_URL}/auth/password-reset/confirm`, {
        token,
        new_password: password,
      });
      setDone(true);
      setTimeout(() => navigate('/login'), 2500);
    } catch (err) {
      const ax = err as { response?: { status?: number; data?: { error?: string } } };
      const status = ax.response?.status;
      if (status === 401) {
        setError('Ссылка недействительна или истекла. Запросите новую.');
      } else if (status === 400) {
        setError(ax.response?.data?.error || 'Некорректные данные.');
      } else {
        setError('Не удалось сохранить пароль. Попробуйте позже.');
      }
      console.error('password-reset confirm failed', err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-md space-y-6">
        <h1 className="text-2xl font-bold text-gray-100">Установка нового пароля</h1>

        {done ? (
          <div
            role="status"
            aria-live="polite"
            className="rounded border border-green-700 bg-green-900/30 p-4 text-gray-200"
          >
            Пароль обновлён. Сейчас вас перенаправит на страницу входа…
          </div>
        ) : (
          <form onSubmit={handleSubmit} noValidate>
            <label htmlFor="pw" className="block text-sm text-gray-400 mb-1">
              Новый пароль
            </label>
            <input
              id="pw"
              type="password"
              required
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              aria-describedby="pw-help"
              className="w-full px-3 py-2 rounded bg-gray-800 text-gray-100 border border-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p id="pw-help" className="mt-1 text-xs text-gray-500">
              Минимум 8 символов.
            </p>

            <label htmlFor="pw2" className="mt-4 block text-sm text-gray-400 mb-1">
              Повторите пароль
            </label>
            <input
              id="pw2"
              type="password"
              required
              autoComplete="new-password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              className="w-full px-3 py-2 rounded bg-gray-800 text-gray-100 border border-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />

            {error && (
              <p role="alert" className="mt-2 text-sm text-red-400">
                {error}
              </p>
            )}

            <button
              type="submit"
              disabled={loading}
              className="mt-4 w-full px-4 py-2 rounded bg-blue-600 hover:bg-blue-700 disabled:opacity-60 transition-colors text-white"
            >
              {loading ? 'Сохраняем...' : 'Сохранить пароль'}
            </button>
          </form>
        )}

        <div className="text-sm text-gray-400 text-center">
          <Link to="/login" className="underline hover:text-gray-200">
            Вернуться к входу
          </Link>
        </div>
      </div>
    </div>
  );
}
