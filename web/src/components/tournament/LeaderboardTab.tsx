import { useState } from 'react';
import { ArrowsExpandIcon, ChartBarIcon } from '../icons';
import { WinnersPodium } from './WinnersPodium';
import type { CrossGameLeaderboardEntry, Game } from '../../types';

// Leaderboard Tab Component
export function LeaderboardTab({
  crossGameEntries,
  games,
  isConnected,
  showCrossGame,
  onShowCrossGameChange,
  onToggleFullscreen,
  onRefresh,
  isRefreshing,
  hasActiveMatches,
  isCompleted,
}: {
  crossGameEntries: CrossGameLeaderboardEntry[];
  games: Game[];
  isConnected: boolean;
  showCrossGame: boolean;
  onShowCrossGameChange: (value: boolean) => void;
  onToggleFullscreen: () => void;
  onRefresh: () => void;
  isRefreshing: boolean;
  hasActiveMatches: boolean;
  isCompleted: boolean;
}) {
  // Поллинг каждые 2с удалён: живые данные приходят через WS-инвалидации
  // (useTournamentLive) либо fallback-поллинг TanStack Query на уровне страницы.
  // Флаг оставлен для индикатора «Обновление...» в шапке вкладки.
  const [autoRefresh, setAutoRefresh] = useState(true);

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div className="flex items-center gap-3">
          <h2 className="text-xl font-bold text-gray-100">Рейтинг</h2>
          {isConnected && (
            <span className="online-indicator">
              Онлайн
            </span>
          )}
          {hasActiveMatches && autoRefresh && (
            <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded-full bg-blue-900/30 text-blue-400 text-xs">
              <span className="w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
              Обновление...
            </span>
          )}
          {isRefreshing && (
            <div className="w-4 h-4 border-2 border-primary-800 border-t-primary-400 rounded-full animate-spin" />
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {hasActiveMatches && (
            <button
              onClick={() => setAutoRefresh(!autoRefresh)}
              className={`btn text-sm ${autoRefresh ? 'btn-primary' : 'btn-secondary'}`}
            >
              {autoRefresh ? 'Авто-обновление вкл' : 'Авто-обновление выкл'}
            </button>
          )}
          <button
            onClick={onRefresh}
            disabled={isRefreshing}
            className="btn btn-secondary text-sm"
          >
            Обновить
          </button>
          <button
            onClick={() => onShowCrossGameChange(true)}
            className={`btn ${showCrossGame ? 'btn-primary' : 'btn-secondary'}`}
          >
            По играм
          </button>
          <button
            onClick={() => onShowCrossGameChange(false)}
            className={`btn ${!showCrossGame ? 'btn-primary' : 'btn-secondary'}`}
          >
            Общий
          </button>
          <button onClick={onToggleFullscreen} className="btn btn-secondary">
            <ArrowsExpandIcon />
            На весь экран
          </button>
        </div>
      </div>

      {/* Show animated podium for completed tournaments */}
      {isCompleted && crossGameEntries.length >= 3 && (
        <WinnersPodium entries={crossGameEntries} />
      )}

      {showCrossGame ? (
        <CrossGameLeaderboardTable entries={crossGameEntries} games={games} />
      ) : (
        <GeneralLeaderboardTable entries={crossGameEntries} />
      )}
    </div>
  );
}

// General Leaderboard Table Component - uses CrossGameLeaderboardEntry data
// Shows: rank, team name, total score, games played, score per game
export function GeneralLeaderboardTable({
  entries,
  isDark = false,
}: {
  entries: CrossGameLeaderboardEntry[];
  isDark?: boolean;
}) {
  if (entries.length === 0) {
    return (
      <div className={`empty-state ${isDark ? 'text-gray-400' : ''}`}>
        <div className="empty-state-icon">
          <ChartBarIcon />
        </div>
        <h3 className="empty-state-title">Пока нет результатов</h3>
        <p className="empty-state-description">
          Таблица обновится после завершения матчей
        </p>
      </div>
    );
  }

  // Find max score for visual bars
  const maxScore = Math.max(...entries.map(e => e.total_rating), 1);

  const getRankBadge = (rank: number) => {
    if (rank === 1) {
      return (
        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-yellow-300 to-amber-500 flex items-center justify-center shadow-lg shadow-amber-500/30">
          <svg className="w-5 h-5 text-amber-900" fill="currentColor" viewBox="0 0 20 20">
            <path fillRule="evenodd" d="M5 5V.13a2.96 2.96 0 0 0-1.293.749L.879 3.707A2.96 2.96 0 0 0 .13 5H5Zm1.5 6.5H2a2 2 0 0 0-2 2v3a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2v-3a2 2 0 0 0-2-2H6.5ZM6 9a2 2 0 1 0 0-4 2 2 0 0 0 0 4Zm7.5 2.5H18a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2h-4.5v-7Zm1.5-6a2 2 0 1 0 0 4 2 2 0 0 0 0-4Z" clipRule="evenodd"/>
          </svg>
        </div>
      );
    }
    if (rank === 2) {
      return (
        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-gray-200 to-gray-400 flex items-center justify-center shadow-lg shadow-gray-500/20">
          <span className="font-bold text-gray-700">2</span>
        </div>
      );
    }
    if (rank === 3) {
      return (
        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-orange-300 to-orange-500 flex items-center justify-center shadow-lg shadow-orange-500/20">
          <span className="font-bold text-orange-900">3</span>
        </div>
      );
    }
    return (
      <div className={`w-10 h-10 rounded-full flex items-center justify-center font-bold ${
        isDark ? 'bg-gray-700 text-gray-300' : 'bg-gray-800 text-gray-300'
      }`}>
        {rank}
      </div>
    );
  };

  const getRowClass = (index: number) => {
    if (index === 0) return isDark ? 'bg-amber-900/10' : 'bg-amber-900/10';
    if (index === 1) return isDark ? 'bg-gray-700/20' : 'bg-gray-700/20';
    if (index === 2) return isDark ? 'bg-orange-900/10' : 'bg-orange-900/10';
    return '';
  };

  return (
    <div className={isDark
      ? 'grid grid-cols-2 xl:grid-cols-3 gap-2'
      : 'space-y-2'
    }>
      {/* Card-style entries */}
      {entries.map((entry, index) => (
        <div
          key={entry.program_id}
          className={`${isDark ? 'p-2.5' : 'p-4'} rounded-xl transition-colors ${
            isDark
              ? `bg-gray-800/50 border border-gray-700 ${getRowClass(index)}`
              : `bg-gray-800/50 border border-gray-800 ${getRowClass(index)} hover:shadow-md`
          }`}
        >
          <div className={`flex items-center ${isDark ? 'gap-3' : 'gap-4'}`}>
            {/* Rank */}
            {isDark ? (
              <div className={`w-7 h-7 rounded-full flex items-center justify-center text-sm font-bold shrink-0 ${
                index === 0 ? 'bg-gradient-to-br from-yellow-300 to-amber-500 text-amber-900' :
                index === 1 ? 'bg-gradient-to-br from-gray-200 to-gray-400 text-gray-700' :
                index === 2 ? 'bg-gradient-to-br from-orange-300 to-orange-500 text-orange-900' :
                'bg-gray-700 text-gray-300'
              }`}>
                {entry.rank}
              </div>
            ) : getRankBadge(entry.rank)}

            {/* Team Info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <h3 className={`font-bold truncate ${isDark ? 'text-sm text-white' : 'text-lg text-gray-100'}`}>
                    {entry.team_name || entry.program_name}
                  </h3>
                  {!isDark && (
                    <div className="flex items-center gap-3 text-sm text-gray-400">
                      <span>{entry.total_games} игр</span>
                      <span>•</span>
                      <span className="text-emerald-400">{entry.total_wins}W</span>
                      <span className="text-red-400">{entry.total_losses}L</span>
                    </div>
                  )}
                </div>

                {/* Score */}
                <div className="text-right shrink-0">
                  <div className={`font-bold tabular-nums ${isDark ? 'text-xl' : 'text-3xl'} ${
                    index === 0 ? 'text-amber-500' :
                    index === 1 ? 'text-gray-400' :
                    index === 2 ? 'text-orange-500' :
                    'text-primary-400'
                  }`}>
                    {entry.total_rating.toLocaleString()}
                  </div>
                  {!isDark && (
                    <div className="text-xs text-gray-400">
                      очков
                    </div>
                  )}
                </div>
              </div>

              {/* Score bar - только в обычном режиме */}
              {!isDark && (
                <div className="mt-3 h-2 bg-gray-700 rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full transition-[width] duration-500 ${
                      index === 0 ? 'bg-gradient-to-r from-amber-400 to-amber-500' :
                      index === 1 ? 'bg-gradient-to-r from-gray-500 to-gray-600' :
                      index === 2 ? 'bg-gradient-to-r from-orange-400 to-orange-500' :
                      'bg-gradient-to-r from-primary-400 to-primary-500'
                    }`}
                    style={{ width: `${(entry.total_rating / maxScore) * 100}%` }}
                  />
                </div>
              )}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

// Cross-Game Leaderboard Table Component
function CrossGameLeaderboardTable({
  entries,
  games,
  isDark = false,
  isCompact = false,
}: {
  entries: CrossGameLeaderboardEntry[];
  games: Game[];
  isDark?: boolean;
  isCompact?: boolean;
}) {
  if (entries.length === 0) {
    return (
      <div className={`empty-state ${isDark ? 'text-gray-400' : ''}`}>
        <div className="empty-state-icon">
          <ChartBarIcon />
        </div>
        <h3 className="empty-state-title">Пока нет результатов</h3>
        <p className="empty-state-description">
          Таблица обновится после завершения матчей
        </p>
      </div>
    );
  }

  const getRankClass = (index: number) => {
    if (index === 0) return 'rank-badge rank-gold';
    if (index === 1) return 'rank-badge rank-silver';
    if (index === 2) return 'rank-badge rank-bronze';
    return isDark ? 'rank-badge bg-gray-700 text-gray-300' : 'rank-badge rank-default';
  };

  const getRowClass = (index: number) => {
    if (index === 0) return isDark ? 'bg-amber-900/20' : 'leaderboard-row-gold';
    if (index === 1) return isDark ? 'bg-gray-700/30' : 'leaderboard-row-silver';
    if (index === 2) return isDark ? 'bg-orange-900/20' : 'leaderboard-row-bronze';
    return '';
  };

  const cellPx = isCompact ? 'px-3 py-1' : 'px-4 py-3';
  const headPx = isCompact ? 'px-3 py-1.5' : 'px-4 py-3';
  const headText = isCompact ? 'text-[10px]' : 'text-sm';

  return (
    <div className={`overflow-x-auto ${isDark ? '' : 'card p-0'}`}>
      <table className={`w-full ${isDark ? 'text-white' : 'text-gray-100'} ${isCompact ? 'text-sm' : ''}`}>
        <thead className={isDark ? 'bg-gray-800/50' : 'bg-gray-800/50'}>
          <tr>
            <th className={`${headPx} text-left font-semibold ${headText} uppercase tracking-wide`}>Место</th>
            <th className={`${headPx} text-left font-semibold ${headText} uppercase tracking-wide`}>Команда</th>
            {games.map((game) => (
              <th key={game.id} className={`${headPx} text-center font-semibold ${headText} uppercase tracking-wide`}>
                {game.display_name}
              </th>
            ))}
            <th className={`${headPx} text-right font-semibold ${headText} uppercase tracking-wide`}>Сумма</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((entry, index) => (
            <tr
              key={entry.program_id}
              className={`border-b ${isDark ? 'border-gray-700/50' : 'border-gray-700'} ${getRowClass(index)} transition-colors`}
            >
              <td className={cellPx}>
                <span className={getRankClass(index)}>
                  {entry.rank}
                </span>
              </td>
              <td className={cellPx}>
                <span className="font-semibold">
                  {entry.team_name || entry.program_name}
                </span>
              </td>
              {games.map((game) => {
                const gameRating = entry.game_ratings[game.id];
                return (
                  <td key={game.id} className={`${cellPx} text-center`}>
                    {gameRating ? (
                      <div>
                        <span className="font-mono font-bold">{Math.round(gameRating.rating)}</span>
                        {!isCompact && (
                          <div className={`text-xs ${isDark ? 'text-gray-400' : 'text-gray-400'}`}>
                            <span className="text-emerald-500" title="Побед">{gameRating.wins}</span>
                            <span className="mx-0.5">/</span>
                            <span className="text-red-500" title="Поражений">{gameRating.losses}</span>
                            <span className="mx-0.5">/</span>
                            <span title="Ничьих">{gameRating.draws || 0}</span>
                          </div>
                        )}
                      </div>
                    ) : (
                      <span className="text-gray-400">-</span>
                    )}
                  </td>
                );
              })}
              <td className={`${cellPx} text-right`}>
                <span className={`font-mono font-bold ${isCompact ? 'text-base' : 'text-lg'} ${isDark ? 'text-primary-400' : 'text-primary-400'}`}>
                  {entry.total_rating}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// Dark mode alias for fullscreen
export function CrossGameLeaderboardTableDark({
  entries,
  games,
}: {
  entries: CrossGameLeaderboardEntry[];
  games: Game[];
}) {
  return <CrossGameLeaderboardTable entries={entries} games={games} isDark isCompact />;
}
