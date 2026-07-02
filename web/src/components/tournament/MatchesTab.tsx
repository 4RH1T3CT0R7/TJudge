import { useState } from 'react';
import { FolderIcon, ChevronDownIcon, ChevronRightIcon } from '../icons';
import type { MatchRound } from '../../types';

// Matches Tab Component - отображает матчи, сгруппированные по раундам
export function MatchesTab({
  rounds,
  onRefresh,
  isRefreshing,
  isAdmin
}: {
  rounds: MatchRound[];
  onRefresh: () => void;
  isRefreshing: boolean;
  isAdmin: boolean;
}) {
  const [expandedRounds, setExpandedRounds] = useState<Set<string>>(new Set());
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [hiddenRounds, setHiddenRounds] = useState<Set<string>>(new Set());

  const hideRound = (roundKey: string) => {
    setHiddenRounds(prev => {
      const next = new Set(prev);
      next.add(roundKey);
      return next;
    });
  };

  const showAllRounds = () => setHiddenRounds(new Set());

  // Проверяем, есть ли активные матчи (pending или running)
  // Поллинг каждые 2с удалён: живые данные приходят через WS-инвалидации
  // (useTournamentLive) либо fallback-поллинг TanStack Query на уровне страницы.
  const hasActiveMatches = rounds.some(
    r => r.pending_count > 0 || r.running_count > 0
  );

  const toggleRound = (roundKey: string) => {
    setExpandedRounds(prev => {
      const next = new Set(prev);
      if (next.has(roundKey)) {
        next.delete(roundKey);
      } else {
        next.add(roundKey);
      }
      return next;
    });
  };

  const expandAll = () => {
    setExpandedRounds(new Set(rounds.map(r => `${r.round_number}-${r.game_type}`)));
  };

  const collapseAll = () => {
    setExpandedRounds(new Set());
  };

  if (rounds.length === 0) {
    return (
      <div className="empty-state">
        <div className="empty-state-icon">
          <FolderIcon />
        </div>
        <h3 className="empty-state-title">Нет матчей</h3>
        <p className="empty-state-description">
          Матчи появятся после запуска раундов
        </p>
      </div>
    );
  }

  // Суммарная статистика по всем раундам
  const totalStats = rounds.reduce(
    (acc, round) => ({
      total: acc.total + round.total_matches,
      completed: acc.completed + round.completed_count,
      pending: acc.pending + round.pending_count,
      running: acc.running + round.running_count,
      failed: acc.failed + round.failed_count,
    }),
    { total: 0, completed: 0, pending: 0, running: 0, failed: 0 }
  );

  return (
    <div>
      {/* Header with summary stats */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div>
          <div className="flex items-center gap-3 mb-2">
            <h2 className="text-xl font-bold text-gray-100">
              Матчи по раундам
            </h2>
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
          <div className="flex flex-wrap gap-3 text-sm">
            <span className="text-gray-300">
              Всего: <strong className="text-gray-100">{totalStats.total}</strong>
            </span>
            <span className="text-emerald-400">
              Завершено: <strong>{totalStats.completed}</strong>
            </span>
            {totalStats.running > 0 && (
              <span className="text-blue-400">
                Выполняется: <strong>{totalStats.running}</strong>
              </span>
            )}
            {totalStats.pending > 0 && (
              <span className="text-yellow-400">
                В очереди: <strong>{totalStats.pending}</strong>
              </span>
            )}
            {totalStats.failed > 0 && (
              <span className="text-red-400">
                Ошибки: <strong>{totalStats.failed}</strong>
              </span>
            )}
          </div>
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
          <button onClick={expandAll} className="btn btn-secondary text-sm">
            Развернуть все
          </button>
          <button onClick={collapseAll} className="btn btn-secondary text-sm">
            Свернуть все
          </button>
        </div>
      </div>

      {/* Overall progress bar */}
      {totalStats.total > 0 && (
        <div className="mb-6 card p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-300">
              Общий прогресс
            </span>
            <span className="text-sm font-mono text-gray-300">
              {totalStats.completed} / {totalStats.total} ({Math.round((totalStats.completed / totalStats.total) * 100)}%)
            </span>
          </div>
          <div className="w-full h-4 bg-gray-700 rounded-full overflow-hidden">
            <div className="h-full flex">
              {/* Completed - green */}
              <div
                className="bg-emerald-500 transition-[width] duration-500"
                style={{ width: `${(totalStats.completed / totalStats.total) * 100}%` }}
              />
              {/* Running - blue, animated */}
              <div
                className="bg-blue-500 animate-pulse transition-[width] duration-500"
                style={{ width: `${(totalStats.running / totalStats.total) * 100}%` }}
              />
              {/* Failed - red */}
              <div
                className="bg-red-500 transition-[width] duration-500"
                style={{ width: `${(totalStats.failed / totalStats.total) * 100}%` }}
              />
            </div>
          </div>
          <div className="flex flex-wrap gap-4 mt-2 text-xs">
            <span className="flex items-center gap-1">
              <span className="w-3 h-3 rounded-full bg-emerald-500" />
              Завершено
            </span>
            {totalStats.running > 0 && (
              <span className="flex items-center gap-1">
                <span className="w-3 h-3 rounded-full bg-blue-500 animate-pulse" />
                Выполняется
              </span>
            )}
            {totalStats.pending > 0 && (
              <span className="flex items-center gap-1">
                <span className="w-3 h-3 rounded-full bg-gray-600" />
                В очереди
              </span>
            )}
            {totalStats.failed > 0 && (
              <span className="flex items-center gap-1">
                <span className="w-3 h-3 rounded-full bg-red-500" />
                Ошибки
              </span>
            )}
          </div>
        </div>
      )}

      {/* Hidden rounds notice */}
      {hiddenRounds.size > 0 && (
        <div className="mb-4 flex items-center gap-3 text-sm text-gray-400">
          <span>Скрыто раундов: {hiddenRounds.size}</span>
          <button onClick={showAllRounds} className="text-primary-400 hover:text-primary-300 underline">
            Показать все
          </button>
        </div>
      )}

      {/* Rounds list */}
      <div className="space-y-3">
        {rounds
          .filter(r => !hiddenRounds.has(`${r.round_number}-${r.game_type}`))
          .map((round) => {
            const roundKey = `${round.round_number}-${round.game_type}`;
            return (
              <RoundCard
                key={roundKey}
                round={round}
                isExpanded={expandedRounds.has(roundKey)}
                onToggle={() => toggleRound(roundKey)}
                isAdmin={isAdmin}
                onHide={() => hideRound(roundKey)}
              />
            );
          })}
      </div>
    </div>
  );
}

// Game name display mapping
const gameDisplayNames: Record<string, string> = {
  dilemma: 'Дилемма заключённого',
  tug_of_war: 'Перетягивание каната',
  travelers_dilemma: 'Дилемма путешественника',
  public_goods: 'Общественное благо',
  dollar_auction: 'Аукцион двойной цены',
};

const getGameDisplayName = (gameType: string) => gameDisplayNames[gameType] || gameType;

// Компонент карточки раунда
function RoundCard({
  round,
  isExpanded,
  onToggle,
  isAdmin,
  onHide,
}: {
  round: MatchRound;
  isExpanded: boolean;
  onToggle: () => void;
  isAdmin: boolean;
  onHide: () => void;
}) {
  const getStatusColor = () => {
    if (round.failed_count > 0) return 'border-l-red-500';
    if (round.running_count > 0) return 'border-l-blue-500';
    if (round.pending_count > 0) return 'border-l-yellow-500';
    if (round.completed_count === round.total_matches) return 'border-l-emerald-500';
    return 'border-l-gray-600';
  };

  const getProgressPercent = () => {
    if (round.total_matches === 0) return 0;
    return Math.round((round.completed_count / round.total_matches) * 100);
  };

  // Подсчёт статистики по победам/ничьим
  const matchStats = round.matches.reduce(
    (acc, match) => {
      if (match.status === 'completed') {
        if (match.winner === 1) acc.wins1++;
        else if (match.winner === 2) acc.wins2++;
        else acc.draws++;
      }
      return acc;
    },
    { wins1: 0, wins2: 0, draws: 0 }
  );

  return (
    <div className={`card p-0 border-l-4 ${getStatusColor()} overflow-hidden`}>
      {/* Round header - collapsible */}
      <button
        onClick={onToggle}
        className="w-full px-4 py-3 flex items-center justify-between hover:bg-gray-800/50 transition-colors"
      >
        <div className="flex items-center gap-3">
          <div className="text-gray-400">
            {isExpanded ? <ChevronDownIcon /> : <ChevronRightIcon />}
          </div>
          <div className="flex items-center gap-2">
            <FolderIcon />
            <span className="font-semibold text-gray-100">
              Раунд {round.round_number}
            </span>
            <span className="px-2 py-0.5 bg-primary-900/30 text-primary-400 text-xs rounded-full font-medium">
              {getGameDisplayName(round.game_type)}
            </span>
          </div>
          <span className="text-sm text-gray-400">
            {round.total_matches} матчей
          </span>
        </div>

        <div className="flex items-center gap-4">
          {/* Mini stats badges */}
          <div className="hidden sm:flex items-center gap-2 text-xs">
            {round.completed_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-emerald-900/30 text-emerald-400">
                {round.completed_count} завершено
              </span>
            )}
            {round.running_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-blue-900/30 text-blue-400">
                {round.running_count} выполняется
              </span>
            )}
            {round.pending_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-yellow-900/30 text-yellow-400">
                {round.pending_count} в очереди
              </span>
            )}
            {round.failed_count > 0 && (
              <span className="px-2 py-1 rounded-full bg-red-900/30 text-red-400">
                {round.failed_count} ошибок
              </span>
            )}
          </div>

          {/* Progress bar */}
          <div className="w-24 h-2 bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-emerald-500 transition-[width] duration-300"
              style={{ width: `${getProgressPercent()}%` }}
            />
          </div>
          <span className="text-sm font-mono text-gray-300 w-12 text-right">
            {getProgressPercent()}%
          </span>
          {isAdmin && round.failed_count > 0 && (
            <button
              onClick={(e) => { e.stopPropagation(); onHide(); }}
              className="ml-2 px-2 py-1 text-xs text-red-400 hover:text-red-300 hover:bg-red-900/30 rounded transition-colors"
              title="Скрыть этот раунд"
            >
              ✕
            </button>
          )}
        </div>
      </button>

      {/* Expanded content */}
      {isExpanded && (
        <div className="border-t border-gray-800">
          {/* Round summary */}
          <div className="px-4 py-3 bg-gray-800/30 flex flex-wrap gap-4 text-sm">
            <span className="text-gray-300">
              Дата: <strong className="text-gray-100">
                {new Date(round.created_at).toLocaleString('ru-RU')}
              </strong>
            </span>
            {round.completed_count > 0 && (
              <>
                <span className="text-emerald-400">
                  Побед P1: <strong>{matchStats.wins1}</strong>
                </span>
                <span className="text-blue-400">
                  Побед P2: <strong>{matchStats.wins2}</strong>
                </span>
                <span className="text-gray-300">
                  Ничьих: <strong>{matchStats.draws}</strong>
                </span>
              </>
            )}
          </div>

          {/* Matches table */}
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-800/50">
                <tr>
                  <th className="px-4 py-2 text-left font-medium text-gray-300">Статус</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-300">Программа 1</th>
                  <th className="px-4 py-2 text-center font-medium text-gray-300">Счёт</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-300">Программа 2</th>
                  <th className="px-4 py-2 text-left font-medium text-gray-300">Игра</th>
                </tr>
              </thead>
              <tbody>
                {round.matches.map((match) => (
                  <MatchRow key={match.id} match={match} />
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

// Компонент строки матча
function MatchRow({ match }: { match: MatchRound['matches'][0] }) {
  const getStatusBadge = () => {
    switch (match.status) {
      case 'completed':
        return <span className="badge badge-green text-xs">Завершён</span>;
      case 'running':
        return <span className="badge badge-blue text-xs">Выполняется</span>;
      case 'pending':
        return <span className="badge badge-yellow text-xs">В очереди</span>;
      case 'failed':
        return <span className="badge badge-red text-xs">Ошибка</span>;
      default:
        return <span className="badge badge-gray text-xs">{match.status}</span>;
    }
  };

  const getScoreDisplay = () => {
    if (match.status !== 'completed') {
      return <span className="text-gray-400">—</span>;
    }

    const score1Class = match.winner === 1 ? 'text-emerald-400 font-bold' : '';
    const score2Class = match.winner === 2 ? 'text-emerald-400 font-bold' : '';

    return (
      <span className="font-mono">
        <span className={score1Class}>{match.score1 ?? 0}</span>
        <span className="text-gray-400 mx-1">:</span>
        <span className={score2Class}>{match.score2 ?? 0}</span>
      </span>
    );
  };

  const getProgram1Class = () => {
    if (match.status !== 'completed') return '';
    return match.winner === 1 ? 'font-semibold text-emerald-400' : '';
  };

  const getProgram2Class = () => {
    if (match.status !== 'completed') return '';
    return match.winner === 2 ? 'font-semibold text-emerald-400' : '';
  };

  return (
    <tr className="border-b border-gray-700 hover:bg-gray-800/30">
      <td className="px-4 py-2">
        {getStatusBadge()}
      </td>
      <td className={`px-4 py-2 ${getProgram1Class()}`}>
        <code className="text-xs bg-gray-800 px-1.5 py-0.5 rounded">
          {match.program1_id.slice(0, 8)}
        </code>
      </td>
      <td className="px-4 py-2 text-center">
        {getScoreDisplay()}
      </td>
      <td className={`px-4 py-2 ${getProgram2Class()}`}>
        <code className="text-xs bg-gray-800 px-1.5 py-0.5 rounded">
          {match.program2_id.slice(0, 8)}
        </code>
      </td>
      <td className="px-4 py-2">
        <span className="text-gray-300">{match.game_type}</span>
      </td>
    </tr>
  );
}
