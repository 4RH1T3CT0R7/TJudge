import { useState } from 'react';
import { Link } from 'react-router-dom';
import { FolderIcon, ChevronDownIcon, ChevronRightIcon } from '../icons';
import { useMatchesByRounds, useRoundMatches, ROUND_PAGE_SIZE } from '../../hooks/queries';
import { StatusLabel } from '../ui/StatusLabel';
import { Spinner } from '../ui/Spinner';
import { EmptyState } from '../ui/EmptyState';
import { Field } from '../ui/Field';
import { YouMark } from '../ui/YouMark';
import { MatchError } from './MatchError';
import type { Side } from '../../utils/explainError';
import type { Match, MatchRound, Team } from '../../types';

// Matches Tab Component - отображает матчи, сгруппированные по раундам.
// rounds - только счётчики; матчи раунда грузятся страницами при раскрытии.
export function MatchesTab({
  tournamentId,
  rounds: allRounds,
  teams = [],
  myTeamId,
  onRefresh,
  isRefreshing,
  isAdmin,
  pollInterval,
}: {
  tournamentId: string;
  rounds: MatchRound[];
  teams?: Team[];
  myTeamId?: string;
  onRefresh: () => void;
  isRefreshing: boolean;
  isAdmin: boolean;
  pollInterval: number | false;
}) {
  const [expandedRounds, setExpandedRounds] = useState<Set<string>>(new Set());
  const [hiddenRounds, setHiddenRounds] = useState<Set<string>>(new Set());
  // Фильтр по команде сужает и счётчики раундов, и страницы матчей
  const [teamId, setTeamId] = useState('');
  const teamRounds = useMatchesByRounds(tournamentId, { teamId, pollInterval, enabled: !!teamId });
  const rounds = teamId ? (teamRounds.data ?? []) : allRounds;

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

  if (allRounds.length === 0) {
    return (
      <EmptyState command="матчи" hint="пока пусто: матчи появятся после запуска раундов" />
    );
  }

  // своя команда первой: её матчи ищут чаще всего
  const teamOptions = [...teams].sort((a, b) => Number(b.id === myTeamId) - Number(a.id === myTeamId));

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
            {hasActiveMatches && (
              <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded bg-blue-900/30 text-blue-400 text-xs">
                <span className="w-2 h-2 bg-blue-500 rounded-full" />
                Обновление...
              </span>
            )}
            {isRefreshing && <Spinner />}
          </div>
          <div className="flex flex-wrap gap-3 text-sm">
            <span className="text-gray-300">
              Всего: <strong className="text-gray-100">{totalStats.total}</strong>
            </span>
            <span className="text-emerald-400">
              Сыграно: <strong>{totalStats.completed}</strong>
            </span>
            {totalStats.running > 0 && (
              <span className="text-blue-400">
                Идёт: <strong>{totalStats.running}</strong>
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
        <div className="flex flex-wrap items-end gap-2">
          {teams.length > 0 && (
            <Field label="Команда">
              {(control) => (
                <select
                  {...control}
                  value={teamId}
                  onChange={(e) => setTeamId(e.target.value)}
                  className="input w-auto max-w-56"
                >
                  <option value="">все команды</option>
                  {teamOptions.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.id === myTeamId ? `${t.name} (вы)` : t.name}
                    </option>
                  ))}
                </select>
              )}
            </Field>
          )}
          <button
            onClick={onRefresh}
            disabled={isRefreshing}
            className="btn btn-secondary"
          >
            Обновить
          </button>
          <button onClick={expandAll} className="btn btn-secondary">
            Развернуть все
          </button>
          <button onClick={collapseAll} className="btn btn-secondary">
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
                className="bg-emerald-500 transition-[width] duration-(--dur-slow)"
                style={{ width: `${(totalStats.completed / totalStats.total) * 100}%` }}
              />
              {/* Running - blue */}
              <div
                className="bg-blue-500 transition-[width] duration-(--dur-slow)"
                style={{ width: `${(totalStats.running / totalStats.total) * 100}%` }}
              />
              {/* Failed - red */}
              <div
                className="bg-red-500 transition-[width] duration-(--dur-slow)"
                style={{ width: `${(totalStats.failed / totalStats.total) * 100}%` }}
              />
            </div>
          </div>
          <div className="flex flex-wrap gap-4 mt-2 text-xs">
            <span className="flex items-center gap-1">
              <span className="w-3 h-3 rounded-full bg-emerald-500" />
              Сыграно
            </span>
            {totalStats.running > 0 && (
              <span className="flex items-center gap-1">
                <span className="w-3 h-3 rounded-full bg-blue-500" />
                Идёт
              </span>
            )}
            {totalStats.pending > 0 && (
              <span className="flex items-center gap-1">
                <span className="w-3 h-3 rounded-full bg-line" />
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

      {teamId && teamRounds.isPending && (
        <p className="py-6 text-sm text-gray-400"><Spinner>загрузка матчей команды</Spinner></p>
      )}
      {teamId && !teamRounds.isPending && rounds.length === 0 && (
        <EmptyState command="матчи команды" hint="у этой команды пока нет матчей" />
      )}

      {/* Rounds list */}
      <div className="space-y-3">
        {rounds
          .filter(r => !hiddenRounds.has(`${r.round_number}-${r.game_type}`))
          .map((round) => {
            const roundKey = `${round.round_number}-${round.game_type}`;
            return (
              <RoundCard
                key={`${roundKey}-${teamId}`}
                tournamentId={tournamentId}
                pollInterval={pollInterval}
                teamId={teamId || undefined}
                myTeamId={myTeamId}
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
  tournamentId,
  pollInterval,
  teamId,
  myTeamId,
  round,
  isExpanded,
  onToggle,
  isAdmin,
  onHide,
}: {
  tournamentId: string;
  pollInterval: number | false;
  teamId?: string;
  myTeamId?: string;
  round: MatchRound;
  isExpanded: boolean;
  onToggle: () => void;
  isAdmin: boolean;
  onHide: () => void;
}) {
  // Отменённые матчи (очистка очереди, дисквалификация) не входят ни в один
  // счётчик, поэтому раунд закрыт, когда ничего не ждёт и не идёт
  const getStatusColor = () => {
    if (round.failed_count > 0) return 'border-l-red-500';
    if (round.running_count > 0) return 'border-l-blue-500';
    if (round.pending_count > 0) return 'border-l-yellow-500';
    if (round.total_matches > 0) return 'border-l-emerald-500';
    return 'border-l-line';
  };

  const getProgressPercent = () => {
    if (round.total_matches === 0) return 0;
    const done = round.total_matches - round.pending_count - round.running_count;
    return Math.round((done / round.total_matches) * 100);
  };

  // Победы считает сервер по всему раунду, ничьи - остаток завершённых
  const draws = round.completed_count - round.wins1 - round.wins2;

  return (
    <div className={`card p-0 border-l-4 ${getStatusColor()} overflow-hidden`}>
      {/* Round header - collapsible; «✕» рядом с кнопкой, а не внутри неё.
          На узкой ширине заголовок переносится, а не уходит под «✕»;
          кольцо фокуса внутри: карточка с overflow-hidden обрезала бы внешнее */}
      <div className="flex items-center hover:bg-gray-800/50 transition-colors">
        <button
          onClick={onToggle}
          aria-expanded={isExpanded}
          className="flex-1 min-w-0 px-4 py-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-2 text-left focus-visible:outline-offset-[-2px]"
        >
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0">
            <div className="flex items-center gap-2">
              <div className="mr-1 text-gray-400">
                {isExpanded ? <ChevronDownIcon /> : <ChevronRightIcon />}
              </div>
              <FolderIcon />
              <span className="font-semibold text-gray-100">
                Раунд {round.round_number}
              </span>
            </div>
            <span className="px-2 py-0.5 bg-primary-900/30 text-primary-400 text-xs rounded font-medium">
              {getGameDisplayName(round.game_type)}
            </span>
            <span className="ml-1 text-sm text-gray-400">
              {round.total_matches} матчей
            </span>
          </div>

          <div className="flex items-center gap-4">
            {/* Mini stats badges */}
            <div className="hidden sm:flex items-center gap-2 text-xs">
              {round.completed_count > 0 && (
                <span className="px-2 py-1 rounded bg-emerald-900/30 text-emerald-400">
                  {round.completed_count} сыграно
                </span>
              )}
              {round.running_count > 0 && (
                <span className="px-2 py-1 rounded bg-blue-900/30 text-blue-400">
                  {round.running_count} идёт
                </span>
              )}
              {round.pending_count > 0 && (
                <span className="px-2 py-1 rounded bg-yellow-900/30 text-yellow-400">
                  {round.pending_count} в очереди
                </span>
              )}
              {round.failed_count > 0 && (
                <span className="px-2 py-1 rounded bg-red-900/30 text-red-400">
                  {round.failed_count} ошибок
                </span>
              )}
            </div>

            {/* Progress bar */}
            <div className="w-24 h-2 bg-gray-700 rounded-full overflow-hidden">
              <div
                className="h-full bg-emerald-500 transition-[width] duration-(--dur-slow)"
                style={{ width: `${getProgressPercent()}%` }}
              />
            </div>
            <span className="text-sm font-mono text-gray-300 w-12 text-right">
              {getProgressPercent()}%
            </span>
          </div>
        </button>
        {isAdmin && round.failed_count > 0 && (
          <button
            onClick={onHide}
            className="mr-4 px-2 py-1 text-xs text-red-400 hover:text-red-300 hover:bg-red-900/30 rounded transition-colors"
            title="Скрыть этот раунд"
            aria-label={`Скрыть раунд ${round.round_number}`}
          >
            ✕
          </button>
        )}
      </div>

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
                  Побед P1: <strong>{round.wins1}</strong>
                </span>
                <span className="text-blue-400">
                  Побед P2: <strong>{round.wins2}</strong>
                </span>
                <span className="text-gray-300">
                  Ничьих: <strong>{draws}</strong>
                </span>
              </>
            )}
          </div>

          <RoundMatches tournamentId={tournamentId} round={round} pollInterval={pollInterval} teamId={teamId} myTeamId={myTeamId} />
        </div>
      )}
    </div>
  );
}

// Таблица матчей раунда: монтируется только в раскрытом раунде и грузит одну страницу
function RoundMatches({
  tournamentId,
  round,
  pollInterval,
  teamId,
  myTeamId,
}: {
  tournamentId: string;
  round: MatchRound;
  pollInterval: number | false;
  teamId?: string;
  myTeamId?: string;
}) {
  const [page, setPage] = useState(0);
  const [jump, setJump] = useState('');
  const { data: matches = [], isPending } = useRoundMatches(
    tournamentId,
    round.round_number,
    round.game_type,
    page,
    { pollInterval, teamId }
  );
  // при фильтре по команде total_matches уже суженный: страниц сколько у команды
  const pageCount = Math.max(1, Math.ceil(round.total_matches / ROUND_PAGE_SIZE));
  const jumpTo = Number(jump);
  const canJump = Number.isInteger(jumpTo) && jumpTo >= 1 && jumpTo <= pageCount;

  return (
    <>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-800/50">
            <tr>
              <th className="px-4 py-2 text-left font-medium text-gray-300">Статус</th>
              <th className="px-4 py-2 text-left font-medium text-gray-300">Игрок 1</th>
              <th className="px-4 py-2 text-center font-medium text-gray-300">Счёт</th>
              <th className="px-4 py-2 text-left font-medium text-gray-300">Игрок 2</th>
            </tr>
          </thead>
          <tbody>
            {matches.map((match) => (
              <MatchRow key={match.id} match={match} myTeamId={myTeamId} />
            ))}
          </tbody>
        </table>
        {isPending && <p className="px-4 py-3 text-sm text-gray-400"><Spinner>загрузка матчей</Spinner></p>}
      </div>
      {pageCount > 1 && (
        <div className="flex flex-wrap items-center justify-center gap-2 px-4 py-3 border-t border-gray-800">
          <button
            onClick={() => setPage((p) => p - 1)}
            disabled={page === 0}
            className="btn btn-secondary"
          >
            Назад
          </button>
          <span className="text-sm text-gray-400 px-4">
            Страница {page + 1} из {pageCount}
          </span>
          <button
            onClick={() => setPage((p) => p + 1)}
            disabled={page + 1 >= pageCount}
            className="btn btn-secondary"
          >
            Вперёд
          </button>
          <form
            className="flex items-center gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              if (!canJump) return;
              setPage(jumpTo - 1);
              setJump('');
            }}
          >
            <input
              type="number"
              min={1}
              max={pageCount}
              value={jump}
              onChange={(e) => setJump(e.target.value)}
              aria-label={`Номер страницы, от 1 до ${pageCount}`}
              placeholder="№"
              className="input w-20"
            />
            <button type="submit" disabled={!canJump} className="btn btn-secondary">
              Перейти
            </button>
          </form>
        </div>
      )}
    </>
  );
}

// Сторона своей команды в матче; null - матч чужой или команд в ответе нет.
function sideOf(match: Match, teamId?: string): Side | null {
  if (!teamId) return null;
  if (match.team1_id === teamId) return 1;
  if (match.team2_id === teamId) return 2;
  return null;
}

// Цвет счёта стороны: победа зелёная; если матч свой, поражение своей стороны красное.
function scoreTone(match: Match, side: Side, mySide: Side | null) {
  if (match.winner === side) return 'text-emerald-400 font-bold';
  if (mySide === side && match.winner && match.winner !== side) return 'text-red-400';
  return '';
}

// Строка матча: команды вместо id программ, у упавшего - раскрытие с вердиктом.
// «почему?» в первой колонке: на телефоне таблица шире экрана, и кнопка
// в последней колонке оказывалась за краем прокрутки
function MatchRow({ match, myTeamId }: { match: Match; myTeamId?: string }) {
  const [open, setOpen] = useState(false);
  const mySide = sideOf(match, myTeamId);
  const names: [string?, string?] = [match.team1_name ?? undefined, match.team2_name ?? undefined];
  const name = (side: Side) => (
    <span className={mySide === side ? 'font-semibold text-gray-100' : undefined}>
      {names[side - 1] ?? (
        <code className="text-xs bg-gray-800 px-1.5 py-0.5 rounded">
          {(side === 1 ? match.program1_id : match.program2_id).slice(0, 8)}
        </code>
      )}
      {mySide === side && <YouMark />}
    </span>
  );

  return (
    <>
      <tr className={`border-b border-gray-700 hover:bg-gray-800/30 ${mySide ? 'row-mine' : ''}`} aria-current={mySide ? 'true' : undefined}>
        <td className="px-4 py-2">
          <div className="flex items-center gap-2">
            <StatusLabel entity="match" status={match.status} />
            {match.status === 'failed' && (
              <button
                type="button"
                onClick={() => setOpen((v) => !v)}
                aria-expanded={open}
                className="btn btn-sm btn-secondary whitespace-nowrap"
              >
                почему?
              </button>
            )}
          </div>
        </td>
        <td className="px-4 py-2">{name(1)}</td>
        <td className="px-4 py-2 text-center">
          {/* счёт - ссылка на разбор матча с ходами */}
          {match.status === 'completed' ? (
            <Link
              to={`/tournaments/${match.tournament_id}/matches/${match.id}`}
              aria-label={`Ходы матча ${names[0] ?? 'игрок 1'} — ${names[1] ?? 'игрок 2'}, счёт ${match.score1 ?? 0}:${match.score2 ?? 0}`}
              className="inline-block border-b border-line font-mono tabular-nums hover:border-primary-400"
            >
              <span className={scoreTone(match, 1, mySide)}>{match.score1 ?? 0}</span>
              <span className="text-gray-400 mx-1">:</span>
              <span className={scoreTone(match, 2, mySide)}>{match.score2 ?? 0}</span>
            </Link>
          ) : match.status === 'failed' ? (
            <Link
              to={`/tournaments/${match.tournament_id}/matches/${match.id}`}
              className="font-mono text-xs text-primary-400 underline hover:text-primary-300"
            >
              ходы
            </Link>
          ) : (
            <span className="text-gray-400">—</span>
          )}
        </td>
        <td className="px-4 py-2">{name(2)}</td>
      </tr>
      {open && (
        <tr className="border-b border-gray-700">
          <td colSpan={4} className="px-4 py-3">
            {/* прилипает к левому краю прокрутки и не шире видимой части
                (экран минус поля страницы, карточки и ячейки): вердикт читается без прокрутки */}
            <div className="sticky left-4 max-w-[calc(100vw-4.5rem)]">
              <MatchError match={match} mySide={mySide} names={names} />
            </div>
          </td>
        </tr>
      )}
    </>
  );
}
