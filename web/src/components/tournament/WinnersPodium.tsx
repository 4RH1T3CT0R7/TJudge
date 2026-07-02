import { useState, useEffect } from 'react';
import type { CrossGameLeaderboardEntry } from '../../types';

// Animated Podium Component for Tournament Winners
export function WinnersPodium({ entries }: { entries: CrossGameLeaderboardEntry[] }) {
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    // Trigger animation after mount
    const timer = setTimeout(() => setIsVisible(true), 100);
    return () => clearTimeout(timer);
  }, []);

  if (entries.length < 3) return null;

  const first = entries[0];
  const second = entries[1];
  const third = entries[2];

  const podiumData = [
    { entry: second, place: 2, height: 'h-28', delay: 'delay-300', bgGradient: 'from-gray-300 via-gray-200 to-gray-400', textColor: 'text-gray-700', medal: '🥈' },
    { entry: first, place: 1, height: 'h-40', delay: 'delay-100', bgGradient: 'from-yellow-400 via-amber-300 to-yellow-500', textColor: 'text-amber-900', medal: '🥇' },
    { entry: third, place: 3, height: 'h-20', delay: 'delay-500', bgGradient: 'from-orange-400 via-orange-300 to-orange-500', textColor: 'text-orange-900', medal: '🥉' },
  ];

  return (
    <div className="mb-8 p-6 bg-gradient-to-b from-primary-900/30 via-primary-800/20 to-transparent rounded-2xl">
      <div className="text-center mb-6">
        <h3 className="text-2xl font-bold text-gray-100 mb-1">
          🏆 Победители турнира 🏆
        </h3>
        <p className="text-gray-400">Поздравляем финалистов!</p>
      </div>

      <div className="flex items-end justify-center gap-4 max-w-2xl mx-auto">
        {podiumData.map(({ entry, place, height, delay, bgGradient, textColor, medal }) => (
          <div
            key={place}
            className={`flex-1 max-w-48 transition-[transform,opacity] duration-700 ease-out ${
              isVisible ? 'opacity-100 translate-y-0' : 'opacity-0 translate-y-10'
            } ${delay}`}
          >
            {/* Winner card */}
            <div className={`text-center mb-2 transform transition-[transform,opacity] duration-500 ${
              isVisible ? 'scale-100' : 'scale-0'
            } ${delay}`}>
              <div className="text-4xl mb-2 animate-bounce" style={{ animationDelay: `${(place - 1) * 200}ms`, animationDuration: '2s' }}>
                {medal}
              </div>
              <div className="font-bold text-lg text-gray-100 truncate px-2">
                {entry.team_name || entry.program_name}
              </div>
              <div className="text-2xl font-bold bg-gradient-to-r from-primary-600 to-primary-400 bg-clip-text text-transparent">
                {entry.total_rating.toLocaleString()}
              </div>
              <div className="text-xs text-gray-400">
                {entry.total_wins}W / {entry.total_losses}L
              </div>
            </div>

            {/* Podium */}
            <div
              className={`${height} bg-gradient-to-t ${bgGradient} rounded-t-lg shadow-lg relative overflow-hidden transition-[transform,opacity] duration-700 ease-out ${
                isVisible ? 'opacity-100' : 'opacity-0'
              } ${delay}`}
            >
              <div className="absolute inset-0 bg-white/20 animate-pulse" style={{ animationDuration: '3s' }} />
              <div className={`absolute inset-x-0 bottom-0 flex items-center justify-center pb-2 ${textColor}`}>
                <span className="text-3xl font-black">{place}</span>
              </div>
              {/* Shine effect */}
              <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/30 to-transparent -translate-x-full animate-shine" />
            </div>
          </div>
        ))}
      </div>

      {/* Confetti-like decorative elements */}
      <div className="relative h-8 overflow-hidden">
        {[...Array(20)].map((_, i) => (
          <div
            key={i}
            className={`absolute w-2 h-2 rounded-full animate-confetti`}
            style={{
              left: `${5 + i * 5}%`,
              backgroundColor: ['#fbbf24', '#a3a3a3', '#fb923c', '#22c55e', '#3b82f6'][i % 5],
              animationDelay: `${i * 0.1}s`,
              animationDuration: `${2 + (i * 0.37) % 1}s`,
            }}
          />
        ))}
      </div>
    </div>
  );
}
