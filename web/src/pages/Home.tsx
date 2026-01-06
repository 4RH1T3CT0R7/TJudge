import { Link } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';

export function Home() {
  const { isAuthenticated } = useAuthStore();

  return (
    <div className="text-center py-12">
      <h1 className="text-4xl font-bold text-gray-900 mb-4">
        Welcome to TJudge
      </h1>
      <p className="text-xl text-gray-600 mb-8 max-w-2xl mx-auto">
        Tournament management system for game theory competitions.
        Create teams, upload programs, and compete in round-robin tournaments.
      </p>

      <div className="flex justify-center space-x-4">
        <Link to="/tournaments" className="btn btn-primary">
          Browse Tournaments
        </Link>
        {!isAuthenticated && (
          <Link to="/register" className="btn btn-secondary">
            Get Started
          </Link>
        )}
      </div>

      {/* Features section */}
      <div className="mt-16 grid grid-cols-1 md:grid-cols-3 gap-8">
        <div className="card">
          <h3 className="text-lg font-semibold mb-2">Team Competition</h3>
          <p className="text-gray-600">
            Create or join teams with unique invite codes.
            Collaborate with teammates on your strategies.
          </p>
        </div>

        <div className="card">
          <h3 className="text-lg font-semibold mb-2">Real-time Leaderboard</h3>
          <p className="text-gray-600">
            Watch your ranking update in real-time as matches complete.
            Support for fullscreen presentation mode.
          </p>
        </div>

        <div className="card">
          <h3 className="text-lg font-semibold mb-2">Multiple Games</h3>
          <p className="text-gray-600">
            Tournaments can include multiple games.
            View rules in Markdown format.
          </p>
        </div>
      </div>
    </div>
  );
}
