import { useState, useCallback, useEffect, useRef } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import axios from 'axios';
import { useAuthStore } from '../store/authStore';
import { SpaceInvader } from '../components/SpaceInvader';
import { CinematicOverlay } from '../components/CinematicOverlay';

const GREETINGS = [
  'console.log("привет!")',
  '// добро пожаловать',
  '{ статус: "онлайн" }',
  'print("привет, мир!")',
  'echo "с возвращением"',
  '// рад тебя видеть',
];

// Текст ошибки входа по ответу сервера. retryAfter - секунды до повтора при 429
function describeLoginError(err: unknown): { text: string; code: string; retryAfter?: number } {
  if (!axios.isAxiosError(err) || !err.response) {
    return { text: '// нет связи с сервером, проверьте сеть', code: 'сети' };
  }
  const { status, headers } = err.response;
  if (status === 401) return { text: '// неверный логин или пароль', code: '401' };
  if (status === 429) {
    const retryAfter = Number(headers['retry-after']) || 60;
    return { text: `// слишком много попыток входа, подождите ${retryAfter} с`, code: '429', retryAfter };
  }
  if (status >= 500) return { text: '// сервер недоступен, попробуйте через минуту', code: String(status) };
  return { text: `// не удалось войти (код ${status})`, code: String(status) };
}

export function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  // позиция каретки в поле пароля, null - поле не в фокусе
  const [passwordCaret, setPasswordCaret] = useState<number | null>(null);
  // ошибка входа держится до правки полей, 429 - до конца отсчёта
  const [error, setError] = useState('');
  const [retryAt, setRetryAt] = useState<number | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const [validationError, setValidationError] = useState<string | null>(null);
  const [focusedField, setFocusedField] = useState<'username' | 'password' | null>(null);
  const [shakeInvader, setShakeInvader] = useState(false);
  const [speechBubble, setSpeechBubble] = useState<string | null>(null);
  const speechTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [showCinematic, setShowCinematic] = useState(false);
  const { login, isLoading } = useAuthStore();
  const navigate = useNavigate();
  // ProtectedRoute кладёт сюда страницу, с которой отправил на логин
  const from = (useLocation().state as { from?: string } | null)?.from ?? '/';

  const monoFont = { fontFamily: "'JetBrains Mono', 'Fira Code', Consolas, monospace" } as const;

  // Greeting on mount
  useEffect(() => {
    const greeting = GREETINGS[Math.floor(Math.random() * GREETINGS.length)];
    setSpeechBubble(greeting);
    speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 4000);
    return () => clearTimeout(speechTimerRef.current);
  }, []);

  // React to focus changes
  useEffect(() => {
    if (focusedField === 'username') {
      clearTimeout(speechTimerRef.current);
      setSpeechBubble('// представься');
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 4000);
    } else if (focusedField === 'password') {
      clearTimeout(speechTimerRef.current);
      setSpeechBubble('// не подглядываю');
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 4000);
    }
  }, [focusedField]);

  // Hearts on username typing
  useEffect(() => {
    if (username.length > 0 && username.length % 5 === 0) {
      clearTimeout(speechTimerRef.current);
      setSpeechBubble('<3');
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 2000);
    }
  }, [username]);

  useEffect(() => {
    if (retryAt === null) return;
    const id = setInterval(() => {
      const t = Date.now();
      setNow(t);
      if (t >= retryAt) {
        setRetryAt(null);
        setError('');
      }
    }, 1000);
    return () => clearInterval(id);
  }, [retryAt]);
  const retryIn = retryAt !== null ? Math.max(0, Math.ceil((retryAt - now) / 1000)) : 0;

  // правка поля снимает ошибку входа, кроме отсчёта после 429
  const clearFieldError = () => {
    setValidationError(null);
    if (retryAt === null) setError('');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (retryAt !== null) return;
    setError('');
    setValidationError(null);

    // Custom validation
    if (!username.trim()) {
      setValidationError('// введите имя пользователя');
      return;
    }
    if (!password) {
      setValidationError('// введите пароль');
      return;
    }

    clearTimeout(speechTimerRef.current);
    setSpeechBubble('// проверяю...');

    try {
      await login(username, password);
      clearTimeout(speechTimerRef.current);

      const currentUser = useAuthStore.getState().user;
      const cinematicKey = currentUser ? `cinematic_first_login_${currentUser.id}` : null;

      if (currentUser && cinematicKey && !localStorage.getItem(cinematicKey)) {
        localStorage.setItem(cinematicKey, '1');
        setShowCinematic(true);
      } else {
        navigate(from);
      }
    } catch (err) {
      const { text, code, retryAfter } = describeLoginError(err);
      setError(text);
      if (retryAfter) {
        const t = Date.now();
        setNow(t);
        setRetryAt(t + retryAfter * 1000);
      }
      setSpeechBubble(`// ошибка ${code}`);
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 3000);
      setShakeInvader(true);
      setTimeout(() => setShakeInvader(false), 600);
    }
  };

  const handleCinematicComplete = useCallback(() => {
    navigate(from);
  }, [navigate, from]);

  // Eye override based on focused field
  const getEyeOverride = useCallback(() => {
    if (focusedField === 'password') return 'closed' as const;
    return null;
  }, [focusedField]);

  const wrapperFocus = (el: HTMLElement | null) => {
    if (el) {
      el.style.borderColor = '#8b5cf6';
      el.style.background = 'rgba(139,92,246,0.06)';
    }
  };

  const wrapperBlur = (el: HTMLElement | null) => {
    if (el) {
      el.style.borderColor = '#374151';
      el.style.background = 'transparent';
    }
  };

  if (showCinematic) {
    return (
      <CinematicOverlay
        type="first_login"
        username={useAuthStore.getState().user?.username}
        onComplete={handleCinematicComplete}
      />
    );
  }

  return (
    <div className="flex-1 flex flex-col items-center justify-center pb-12">
      <div className="w-full max-w-sm mx-auto py-2">
        {/* Invader - z-index выше фиксированного header (z-50), чтобы speech bubble не обрезался */}
        <div className="flex justify-center mb-3 relative z-[60]">
          <SpaceInvader
            size="md"
            interactive
            eyeOverride={getEyeOverride()}
            shake={shakeInvader}
            speechBubble={speechBubble}
          />
        </div>

        {/* Header - terminal style */}
        <div className="text-center mb-5">
          <p
            id="login-error"
            role={error || validationError ? 'alert' : undefined}
            aria-live="assertive"
            aria-atomic="true"
            className={`text-sm mb-1 transition-colors duration-300 ${error ? 'text-red-400' : validationError ? 'text-primary-400' : 'text-gray-500'}`}
            style={monoFont}
          >
            {error ? `stderr: ${error}` : validationError || '// авторизация'}
          </p>
          <h1 className="text-2xl font-bold text-gray-100">
            <span className="text-primary-400" style={monoFont}>$ ssh </span>
            <span>tjudge.ru</span>
          </h1>
        </div>

        <form onSubmit={handleSubmit} noValidate className="space-y-4">
          {/* Username */}
          <div>
            <label htmlFor="username" className="block text-sm text-gray-500 mb-1" style={monoFont}>
              {'// имя пользователя'}
            </label>
            <div
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg transition-[border-color,background-color] duration-200"
              style={{ border: '1px solid #374151', background: 'transparent' }}
            >
              <span className="text-green-400 text-sm shrink-0" style={monoFont}>$</span>
              <input
                type="text"
                id="username"
                value={username}
                onChange={(e) => { setUsername(e.target.value); clearFieldError(); }}
                onFocus={(e) => {
                  setFocusedField('username');
                  wrapperFocus(e.currentTarget.parentElement);
                }}
                onBlur={(e) => {
                  setFocusedField(null);
                  wrapperBlur(e.currentTarget.parentElement);
                }}
                className="flex-1 text-gray-100 placeholder:text-gray-600 bg-transparent"
                style={{ border: 'none', boxShadow: 'none', ...monoFont }}
                autoComplete="username"
                placeholder="username"
                required
                aria-required="true"
                aria-invalid={!!validationError && validationError.includes('имя')}
                aria-describedby={validationError ? 'login-error' : undefined}
              />
            </div>
          </div>

          {/* Password */}
          <div>
            <label htmlFor="password" className="block text-sm text-gray-500 mb-1" style={monoFont}>
              {'// пароль'}
            </label>
            <div
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg transition-[border-color,background-color] duration-200"
              style={{ border: '1px solid #374151', background: 'transparent' }}
            >
              <span className="text-primary-400 text-sm shrink-0" style={monoFont}>{'>'}</span>
              {/* нативное поле пароля (автозаполнение, IME) с прозрачным текстом,
                  поверх - звёздочки и своя каретка: в моноширинном шрифте '*' шириной 1ch */}
              <div className="relative flex-1">
                <input
                  type="password"
                  id="password"
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value);
                    setPasswordCaret(e.target.selectionStart);
                    clearFieldError();
                  }}
                  onSelect={(e) => setPasswordCaret(e.currentTarget.selectionStart)}
                  onFocus={(e) => {
                    setFocusedField('password');
                    setValidationError(null);
                    setPasswordCaret(e.currentTarget.selectionStart);
                    wrapperFocus(e.currentTarget.parentElement?.parentElement ?? null);
                  }}
                  onBlur={(e) => {
                    setFocusedField(null);
                    setPasswordCaret(null);
                    wrapperBlur(e.currentTarget.parentElement?.parentElement ?? null);
                  }}
                  className="w-full text-transparent placeholder:text-gray-600 bg-transparent selection:bg-transparent"
                  style={{ border: 'none', boxShadow: 'none', caretColor: 'transparent', letterSpacing: 0, ...monoFont }}
                  autoComplete="current-password"
                  placeholder="********"
                  aria-label="Пароль"
                  aria-required="true"
                  aria-invalid={!!validationError && validationError.includes('пароль')}
                  aria-describedby={validationError ? 'login-error' : undefined}
                />
                <span
                  aria-hidden="true"
                  className="pointer-events-none absolute inset-0 flex items-center overflow-hidden whitespace-pre text-gray-100"
                  style={{ letterSpacing: 0, ...monoFont }}
                >
                  {'*'.repeat(password.length)}
                  {passwordCaret !== null && (
                    <span className="absolute h-[1.1em] w-px bg-gray-100 animate-pulse" style={{ left: `${passwordCaret}ch` }} />
                  )}
                </span>
              </div>
            </div>
          </div>

          <button
            type="submit"
            disabled={isLoading || retryIn > 0}
            className="w-full btn btn-primary py-2.5"
            style={monoFont}
          >
            {isLoading ? '// загрузка...' : retryIn > 0 ? `// повтор через ${retryIn} с` : 'auth.login()'}
          </button>
        </form>
      </div>
    </div>
  );
}
