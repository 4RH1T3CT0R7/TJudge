import { useState, useCallback, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
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

export function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [validationError, setValidationError] = useState<string | null>(null);
  const [focusedField, setFocusedField] = useState<'username' | 'password' | null>(null);
  const [shakeInvader, setShakeInvader] = useState(false);
  const [jumpInvader, setJumpInvader] = useState(false);
  const [loginSuccess, setLoginSuccess] = useState(false);
  const [speechBubble, setSpeechBubble] = useState<string | null>(null);
  const speechTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const errorTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const [showCinematic, setShowCinematic] = useState(false);
  const { login, isLoading } = useAuthStore();
  const navigate = useNavigate();

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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearTimeout(errorTimerRef.current);
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
        setSpeechBubble('<3');
        setJumpInvader(true);
        setTimeout(() => setJumpInvader(false), 700);
        setTimeout(() => {
          setSpeechBubble('{ доступ: "открыт" }');
          setLoginSuccess(true);
        }, 500);
        setTimeout(() => navigate('/'), 1400);
      }
    } catch {
      setError('// неверный логин или пароль');
      clearTimeout(errorTimerRef.current);
      errorTimerRef.current = setTimeout(() => setError(''), 3000);
      setSpeechBubble('// ошибка 401');
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 3000);
      setShakeInvader(true);
      setTimeout(() => setShakeInvader(false), 600);
    }
  };

  const handleCinematicComplete = useCallback(() => {
    navigate('/');
  }, [navigate]);

  // Eye override based on focused field
  const getEyeOverride = useCallback(() => {
    if (focusedField === 'password') return 'closed' as const;
    return null;
  }, [focusedField]);

  // Password masking — type="text" showing * characters
  const handlePasswordKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Backspace') {
      e.preventDefault();
      setPassword(prev => prev.slice(0, -1));
    } else if (e.key === 'Delete') {
      e.preventDefault();
    } else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      e.preventDefault();
      setPassword(prev => prev + e.key);
    }
  };

  const handlePasswordPaste = (e: React.ClipboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    const pasted = e.clipboardData.getData('text');
    setPassword(prev => prev + pasted);
  };

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
        {/* Invader — z-index above fixed header (z-50) so speech bubble isn't clipped */}
        <div className={`flex justify-center mb-3 relative z-[60] ${loginSuccess ? 'animate-login-success' : ''}`}>
          <SpaceInvader
            size="md"
            interactive
            eyeOverride={loginSuccess ? 'wide' : getEyeOverride()}
            shake={shakeInvader}
            jump={jumpInvader}
            speechBubble={speechBubble}
          />
        </div>

        {/* Header — terminal style */}
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
                onChange={(e) => { setUsername(e.target.value); setValidationError(null); }}
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

          {/* Password — custom asterisk masking */}
          <div>
            <label htmlFor="password" className="block text-sm text-gray-500 mb-1" style={monoFont}>
              {'// пароль'}
            </label>
            <div
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg transition-[border-color,background-color] duration-200"
              style={{ border: '1px solid #374151', background: 'transparent' }}
            >
              <span className="text-primary-400 text-sm shrink-0" style={monoFont}>{'>'}</span>
              {/* Hidden real password input for browser autocomplete */}
              <input
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={() => {}}
                tabIndex={-1}
                aria-hidden="true"
                style={{ position: 'absolute', opacity: 0, width: 0, height: 0, pointerEvents: 'none' }}
              />
              <input
                type="text"
                id="password"
                value={'*'.repeat(password.length)}
                onKeyDown={handlePasswordKeyDown}
                onPaste={handlePasswordPaste}
                onChange={() => {}}
                onFocus={(e) => {
                  setFocusedField('password');
                  setValidationError(null);
                  wrapperFocus(e.currentTarget.parentElement);
                }}
                onBlur={(e) => {
                  setFocusedField(null);
                  wrapperBlur(e.currentTarget.parentElement);
                }}
                className="flex-1 text-gray-100 placeholder:text-gray-600 bg-transparent"
                style={{ border: 'none', boxShadow: 'none', ...monoFont }}
                placeholder="********"
                aria-label="Пароль"
                aria-required="true"
                aria-invalid={!!validationError && validationError.includes('пароль')}
                aria-describedby={validationError ? 'login-error' : undefined}
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="w-full btn btn-primary py-2.5"
            style={monoFont}
          >
            {isLoading ? '// загрузка...' : 'auth.login()'}
          </button>
        </form>
      </div>
    </div>
  );
}
