import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { AxiosError } from 'axios';

interface ApiErrorResponse {
  error?: string;
  message?: string;
}

// Валидация пароля (соответствует бэкенду)
function validatePassword(password: string): string | null {
  if (password.length < 8) {
    return 'Пароль должен быть не менее 8 символов';
  }
  if (password.length > 128) {
    return 'Пароль слишком длинный (максимум 128 символов)';
  }

  const hasUpper = /[A-Z]/.test(password);
  const hasLower = /[a-z]/.test(password);
  const hasDigit = /[0-9]/.test(password);

  if (!hasUpper || !hasLower || !hasDigit) {
    return 'Пароль должен содержать заглавную букву, строчную букву и цифру';
  }

  return null;
}

// Валидация имени пользователя
function validateUsername(username: string): string | null {
  if (username.length < 3) {
    return 'Имя пользователя должно быть не менее 3 символов';
  }
  if (username.length > 50) {
    return 'Имя пользователя слишком длинное (максимум 50 символов)';
  }
  if (!/^[a-zA-Z0-9_-]+$/.test(username)) {
    return 'Имя пользователя может содержать только буквы, цифры, _ и -';
  }
  return null;
}

export function Register() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const { register, isLoading } = useAuthStore();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    // Валидация имени пользователя
    const usernameError = validateUsername(username);
    if (usernameError) {
      setError(usernameError);
      return;
    }

    // Валидация пароля
    const passwordError = validatePassword(password);
    if (passwordError) {
      setError(passwordError);
      return;
    }

    if (password !== confirmPassword) {
      setError('Пароли не совпадают');
      return;
    }

    try {
      await register(username, email, password);
      navigate('/tournaments');
    } catch (err) {
      const axiosError = err as AxiosError<ApiErrorResponse>;
      if (axiosError.response?.data?.error) {
        const serverError = axiosError.response.data.error;
        // Переводим серверные ошибки на русский
        if (serverError.includes('already exists')) {
          setError('Пользователь с таким именем или email уже существует');
        } else if (serverError.includes('password')) {
          setError('Пароль не соответствует требованиям безопасности');
        } else if (serverError.includes('email')) {
          setError('Неверный формат email');
        } else if (serverError.includes('username')) {
          setError('Неверный формат имени пользователя');
        } else {
          setError(serverError);
        }
      } else {
        setError('Ошибка регистрации. Попробуйте снова.');
      }
    }
  };

  return (
    <div className="max-w-md mx-auto">
      <div className="card">
        <h1 className="text-2xl font-bold text-center mb-6 text-gray-900 dark:text-gray-100">Регистрация</h1>

        {error && (
          <div className="bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400 p-3 rounded-lg mb-4">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label htmlFor="username" className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-1">
              Имя пользователя
            </label>
            <input
              type="text"
              id="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="input"
              required
              minLength={3}
            />
          </div>

          <div>
            <label htmlFor="email" className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-1">
              Email
            </label>
            <input
              type="email"
              id="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="input"
              required
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-1">
              Пароль
            </label>
            <input
              type="password"
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="input"
              required
              minLength={8}
            />
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-300">
              Минимум 8 символов, заглавная и строчная буква, цифра
            </p>
          </div>

          <div>
            <label htmlFor="confirmPassword" className="block text-sm font-medium text-gray-700 dark:text-gray-200 mb-1">
              Подтвердите пароль
            </label>
            <input
              type="password"
              id="confirmPassword"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="input"
              required
            />
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="w-full btn btn-primary"
          >
            {isLoading ? 'Регистрация...' : 'Зарегистрироваться'}
          </button>
        </form>

        <p className="mt-4 text-center text-sm text-gray-600 dark:text-gray-300">
          Уже есть аккаунт?{' '}
          <Link to="/login" className="text-primary-600 dark:text-primary-400 hover:text-primary-700 dark:hover:text-primary-300">
            Войти
          </Link>
        </p>
      </div>
    </div>
  );
}
