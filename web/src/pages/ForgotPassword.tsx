import { useState } from 'react';
import { Link } from 'react-router-dom';
import axios from 'axios';

// P1.11: страница запроса восстановления пароля.
// Отправляет email на backend; независимо от результата показывает
// generic-сообщение (user enumeration protection).

const API_BASE_URL = import.meta.env.VITE_API_URL || '/api/v1';

export function ForgotPassword() {
  const [email, setEmail] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email) return;
    setLoading(true);
    setError(null);
    try {
      await axios.post(`${API_BASE_URL}/auth/password-reset/request`, { email });
      setSubmitted(true);
    } catch (err) {
      // Сеть/5xx — показываем реальную ошибку. Для 4xx API возвращает OK
      // даже для несуществующего email, так что сюда не должны попадать.
      setError('Не удалось отправить запрос. Попробуйте позже.');
      console.error('password-reset request failed', err);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center px-4">
      <div className="w-full max-w-md space-y-6">
        <h1 className="text-2xl font-bold text-gray-100">Восстановление пароля</h1>

        {submitted ? (
          <div
            role="status"
            aria-live="polite"
            className="rounded border border-green-700 bg-green-900/30 p-4 text-gray-200"
          >
            Если указанный email зарегистрирован, мы отправили на него ссылку для
            сброса пароля. Проверьте папку "Входящие" и "Спам". Ссылка действительна 1 час.
          </div>
        ) : (
          <form onSubmit={handleSubmit} noValidate>
            <label htmlFor="email" className="block text-sm text-gray-400 mb-1">
              Email
            </label>
            <input
              id="email"
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              aria-describedby={error ? 'email-error' : undefined}
              className="w-full px-3 py-2 rounded bg-gray-800 text-gray-100 border border-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            {error && (
              <p id="email-error" role="alert" className="mt-2 text-sm text-red-400">
                {error}
              </p>
            )}
            <button
              type="submit"
              disabled={loading}
              className="mt-4 w-full px-4 py-2 rounded bg-blue-600 hover:bg-blue-700 disabled:opacity-60 transition-colors text-white"
            >
              {loading ? 'Отправляем...' : 'Отправить ссылку'}
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
