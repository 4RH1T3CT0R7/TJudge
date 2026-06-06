import { useEffect, lazy } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { MotionConfig } from 'motion/react';
import { Layout } from './components/layout/Layout';
import { ErrorBoundary } from './components/ErrorBoundary';
import { ToastContainer } from './components/ToastContainer';
import { useAuthStore } from './store/authStore';
import { InvaderProvider } from './context/InvaderContext';

const pageImports = {
  Home: () => import('./pages/Home'),
  Login: () => import('./pages/Login'),
  Profile: () => import('./pages/Profile'),
  Tournaments: () => import('./pages/Tournaments'),
  TournamentDetail: () => import('./pages/TournamentDetail'),
  GameDetail: () => import('./pages/GameDetail'),
  GameView: () => import('./pages/GameView'),
  Games: () => import('./pages/Games'),
  TeamManagement: () => import('./pages/TeamManagement'),
  AdminPanel: () => import('./pages/AdminPanel'),
  NotFound: () => import('./pages/NotFound'),
};

const Home = lazy(() => pageImports.Home().then(m => ({ default: m.Home })));
const Login = lazy(() => pageImports.Login().then(m => ({ default: m.Login })));
const Profile = lazy(() => pageImports.Profile().then(m => ({ default: m.Profile })));
const Tournaments = lazy(() => pageImports.Tournaments().then(m => ({ default: m.Tournaments })));
const TournamentDetail = lazy(() => pageImports.TournamentDetail().then(m => ({ default: m.TournamentDetail })));
const GameDetail = lazy(() => pageImports.GameDetail().then(m => ({ default: m.GameDetail })));
const GameView = lazy(() => pageImports.GameView().then(m => ({ default: m.GameView })));
const Games = lazy(() => pageImports.Games().then(m => ({ default: m.Games })));
const TeamManagement = lazy(() => pageImports.TeamManagement().then(m => ({ default: m.TeamManagement })));
const AdminPanel = lazy(() => pageImports.AdminPanel().then(m => ({ default: m.AdminPanel })));
const NotFound = lazy(() => pageImports.NotFound().then(m => ({ default: m.NotFound })));

// Prefetch критичных страниц (Navigation targets) через
// requestIdleCallback, чтобы не тормозить LCP на медленных сетях и
// не грузить three.js chunk заранее. three.js прогрузится лениво при
// заходе на главную (через lazy(Home) в существующем коде).
function prefetchCriticalPages() {
  const critical: Array<keyof typeof pageImports> = [
    'Tournaments',
    'Games',
    'Login',
  ];

  const schedule = (cb: () => void) => {
    const ric = (window as unknown as {
      requestIdleCallback?: (cb: () => void, opts?: { timeout?: number }) => number;
    }).requestIdleCallback;
    if (typeof ric === 'function') {
      ric(cb, { timeout: 2000 });
    } else {
      setTimeout(cb, 1500);
    }
  };

  // Не prefetch'им в slow-connection сетях.
  const conn = (navigator as unknown as { connection?: { saveData?: boolean; effectiveType?: string } }).connection;
  if (conn?.saveData) return;
  if (conn?.effectiveType === 'slow-2g' || conn?.effectiveType === '2g') return;

  schedule(() => {
    critical.forEach((key, i) => {
      setTimeout(() => pageImports[key](), i * 100);
    });
  });
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isInitialized } = useAuthStore();

  // Полноэкранный лоадер показываем ТОЛЬКО до первичной инициализации auth.
  // Раньше здесь был ещё `|| isLoading`, из-за чего любое фоновое действие,
  // дёргающее isLoading после init (сохранение профиля и т.п.), мгновенно
  // подменяло всю страницу на «Загрузка...» и обратно — видимое промаргивание.
  if (!isInitialized) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p>Загрузка...</p>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isInitialized, user } = useAuthStore();

  if (!isInitialized) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p>Загрузка...</p>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (user?.role !== 'admin') {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}

function AppContent() {
  const { initialize, isInitialized, isLoading } = useAuthStore();

  useEffect(() => {
    initialize();
  }, [initialize]);

  useEffect(() => {
    prefetchCriticalPages();
  }, []);

  // Show loading while initializing auth
  if (!isInitialized && isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p>Загрузка...</p>
      </div>
    );
  }

  return (
    <Routes>
      <Route path="/" element={<Layout />}>
        <Route index element={<ProtectedRoute><Home /></ProtectedRoute>} />
        <Route path="login" element={<Login />} />
        <Route path="tournaments" element={<Tournaments />} />
        <Route path="tournaments/:id" element={<TournamentDetail />} />
        <Route path="tournaments/:tournamentId/games/:gameId" element={<GameDetail />} />
        <Route path="games" element={<Games />} />
        <Route path="games/:id" element={<GameView />} />
        <Route
          path="teams/:id"
          element={
            <ProtectedRoute>
              <TeamManagement />
            </ProtectedRoute>
          }
        />
        <Route
          path="admin"
          element={
            <AdminRoute>
              <AdminPanel />
            </AdminRoute>
          }
        />
        <Route
          path="profile"
          element={
            <ProtectedRoute>
              <Profile />
            </ProtectedRoute>
          }
        />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}

function App() {
  return (
    <ErrorBoundary>
      <MotionConfig reducedMotion="user">
        <InvaderProvider>
          <BrowserRouter>
            <AppContent />
          </BrowserRouter>
          <ToastContainer />
        </InvaderProvider>
      </MotionConfig>
    </ErrorBoundary>
  );
}

export default App;
