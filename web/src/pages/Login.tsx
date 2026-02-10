import { useState, useCallback, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { SpaceInvader } from '../components/SpaceInvader';

const GREETINGS = [
  '> hello, world!',
  '{ status: "online" }',
  'console.log("привет!")',
  '// добро пожаловать',
  'print("moshi moshi")',
  'echo "welcome"',
];

export function Login() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [focusedField, setFocusedField] = useState<'username' | 'password' | null>(null);
  const [shakeInvader, setShakeInvader] = useState(false);
  const [jumpInvader, setJumpInvader] = useState(false);
  const [speechBubble, setSpeechBubble] = useState<string | null>(null);
  const speechTimerRef = useRef<ReturnType<typeof setTimeout>>(undefined);
  const { login, isLoading } = useAuthStore();
  const navigate = useNavigate();

  // Greeting on mount
  useEffect(() => {
    const greeting = GREETINGS[Math.floor(Math.random() * GREETINGS.length)];
    setSpeechBubble(greeting);
    speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 3000);
    return () => clearTimeout(speechTimerRef.current);
  }, []);

  // React to focus changes
  useEffect(() => {
    clearTimeout(speechTimerRef.current);
    if (focusedField === 'username') {
      setSpeechBubble('> who are you?');
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 2000);
    } else if (focusedField === 'password') {
      setSpeechBubble('// не подглядываю');
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 2000);
    }
  }, [focusedField]);

  // Hearts on username typing
  useEffect(() => {
    if (username.length > 0 && username.length % 5 === 0) {
      clearTimeout(speechTimerRef.current);
      setSpeechBubble('<3');
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 1000);
    }
  }, [username]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    // Show authenticating bubble
    clearTimeout(speechTimerRef.current);
    setSpeechBubble('> authenticating...');

    try {
      await login(username, password);
      // Success
      setSpeechBubble('{ access: "granted" }');
      setJumpInvader(true);
      setTimeout(() => setJumpInvader(false), 100);
      setTimeout(() => navigate('/tournaments'), 800);
    } catch {
      setError('Error: invalid credentials // неверный логин или пароль');
      setSpeechBubble('// ERROR 401');
      speechTimerRef.current = setTimeout(() => setSpeechBubble(null), 3000);
      setShakeInvader(true);
      setTimeout(() => setShakeInvader(false), 100);
    }
  };

  // Eye override based on focused field
  const getEyeOverride = useCallback(() => {
    if (focusedField === 'password') return 'closed' as const;
    return null;
  }, [focusedField]);

  const monoFont = { fontFamily: "'JetBrains Mono', 'Fira Code', Consolas, monospace" } as const;

  const inputBaseStyle = {
    background: 'transparent',
    border: 'none',
    outline: 'none',
    ...monoFont,
  } as const;

  return (
    <div className="min-h-[70vh] flex items-center justify-center">
      <div className="w-full max-w-sm">
        {/* Invader with padding for jump */}
        <div className="flex justify-center mb-6 pt-12">
          <SpaceInvader
            size="md"
            interactive
            eyeOverride={getEyeOverride()}
            shake={shakeInvader}
            jump={jumpInvader}
            speechBubble={speechBubble}
          />
        </div>

        {/* Header — terminal style */}
        <div className="text-center mb-8">
          <p className="text-sm text-gray-500 mb-2" style={monoFont}>{'// авторизация'}</p>
          <h1 className="text-2xl font-bold text-gray-100">
            <span className="text-primary-400" style={monoFont}>$ ssh </span>
            <span>tjudge.io</span>
          </h1>
        </div>

        {error && (
          <div
            className="bg-red-900/20 border border-red-800/30 text-red-400 px-4 py-3 rounded-lg mb-6 text-sm animate-shake"
            style={monoFont}
          >
            <span className="text-red-500">stderr: </span>{error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-5">
          {/* Username */}
          <div>
            <label htmlFor="username" className="block text-sm text-gray-500 mb-1.5" style={monoFont}>
              {'// кто ты, воин?'}
            </label>
            <div
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg transition-all duration-200"
              style={{ border: '1px solid #374151', background: 'transparent' }}
              onFocus={() => setFocusedField('username')}
              onBlur={() => setFocusedField(null)}
            >
              <span className="text-green-400 text-sm shrink-0" style={monoFont}>$</span>
              <input
                type="text"
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                onFocus={(e) => {
                  setFocusedField('username');
                  const wrapper = e.currentTarget.parentElement;
                  if (wrapper) {
                    wrapper.style.borderColor = '#8b5cf6';
                    wrapper.style.boxShadow = '0 0 0 3px rgba(139,92,246,0.15), 0 0 20px rgba(139,92,246,0.3)';
                  }
                }}
                onBlur={(e) => {
                  setFocusedField(null);
                  const wrapper = e.currentTarget.parentElement;
                  if (wrapper) {
                    wrapper.style.borderColor = '#374151';
                    wrapper.style.boxShadow = 'none';
                  }
                }}
                className="flex-1 text-gray-100 placeholder:text-gray-600 bg-transparent outline-none"
                style={inputBaseStyle}
                autoComplete="username"
                required
                placeholder="username"
              />
            </div>
          </div>

          {/* Password */}
          <div>
            <label htmlFor="password" className="block text-sm text-gray-500 mb-1.5" style={monoFont}>
              {'// секретное слово'}
            </label>
            <div
              className="flex items-center gap-2 px-4 py-2.5 rounded-lg transition-all duration-200"
              style={{ border: '1px solid #374151', background: 'transparent' }}
            >
              <span className="text-primary-400 text-sm shrink-0" style={monoFont}>{'>'}</span>
              <input
                type="password"
                id="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                onFocus={(e) => {
                  setFocusedField('password');
                  const wrapper = e.currentTarget.parentElement;
                  if (wrapper) {
                    wrapper.style.borderColor = '#8b5cf6';
                    wrapper.style.boxShadow = '0 0 0 3px rgba(139,92,246,0.15), 0 0 20px rgba(139,92,246,0.3)';
                  }
                }}
                onBlur={(e) => {
                  setFocusedField(null);
                  const wrapper = e.currentTarget.parentElement;
                  if (wrapper) {
                    wrapper.style.borderColor = '#374151';
                    wrapper.style.boxShadow = 'none';
                  }
                }}
                className="flex-1 text-gray-100 placeholder:text-gray-600 bg-transparent outline-none"
                style={inputBaseStyle}
                autoComplete="current-password"
                required
                placeholder="••••••••"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="w-full btn btn-primary py-2.5"
            style={monoFont}
          >
            {isLoading ? '> loading...' : 'auth.login()'}
          </button>
        </form>

      </div>
    </div>
  );
}
