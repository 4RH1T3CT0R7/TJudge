import { useEffect, lazy, Suspense } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { MotionConfig } from 'motion/react';
import { Layout } from './components/layout/Layout';
import { PageLoader } from './components/PageLoader';
import { ErrorBoundary } from './components/ErrorBoundary';
import { useAuthStore } from './store/authStore';
import { InvaderProvider } from './context/InvaderContext';

const Home = lazy(() => import('./pages/Home').then(m => ({ default: m.Home })));
const Login = lazy(() => import('./pages/Login').then(m => ({ default: m.Login })));
const Profile = lazy(() => import('./pages/Profile').then(m => ({ default: m.Profile })));
const Tournaments = lazy(() => import('./pages/Tournaments').then(m => ({ default: m.Tournaments })));
const TournamentDetail = lazy(() => import('./pages/TournamentDetail').then(m => ({ default: m.TournamentDetail })));
const GameDetail = lazy(() => import('./pages/GameDetail').then(m => ({ default: m.GameDetail })));
const GameView = lazy(() => import('./pages/GameView').then(m => ({ default: m.GameView })));
const Games = lazy(() => import('./pages/Games').then(m => ({ default: m.Games })));
const TeamManagement = lazy(() => import('./pages/TeamManagement').then(m => ({ default: m.TeamManagement })));
const AdminPanel = lazy(() => import('./pages/AdminPanel').then(m => ({ default: m.AdminPanel })));
const NotFound = lazy(() => import('./pages/NotFound').then(m => ({ default: m.NotFound })));

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading, isInitialized } = useAuthStore();

  if (!isInitialized || isLoading) {
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
  const { isAuthenticated, isLoading, isInitialized, user } = useAuthStore();

  if (!isInitialized || isLoading) {
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
    // Initialize auth state on app load
    initialize();
  }, [initialize]);

  // Show loading while initializing auth
  if (!isInitialized && isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p>Загрузка...</p>
      </div>
    );
  }

  return (
    <Suspense fallback={<PageLoader />}>
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
    </Suspense>
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
        </InvaderProvider>
      </MotionConfig>
    </ErrorBoundary>
  );
}

export default App;
