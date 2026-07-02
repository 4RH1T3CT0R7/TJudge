import type { Dispatch, SetStateAction } from 'react';
import api from '../../api/client';
import { useToastStore } from '../../store/toastStore';
import { confirmDialog } from '../../store/confirmStore';
import type { FullSystemStatus, Match, MatchStatistics, QueueStats, SystemMetrics } from '../../types';
import type { AdminReactionSetter } from './types';

// Helper function to format bytes to human readable format
const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
};

// Helper function to format uptime to human readable format
const formatUptime = (seconds: number): string => {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  const parts = [];
  if (days > 0) parts.push(`${days}д`);
  if (hours > 0) parts.push(`${hours}ч`);
  if (minutes > 0 || parts.length === 0) parts.push(`${minutes}м`);

  return parts.join(' ');
};

// Возраст в секундах -> «45 с» или «12 мин 30 с» (для outbox.oldest_pending_age_seconds)
const formatAgeSeconds = (seconds: number): string => {
  const total = Math.max(0, Math.round(seconds));
  if (total < 60) return `${total} с`;
  const minutes = Math.floor(total / 60);
  const rest = total % 60;
  return rest > 0 ? `${minutes} мин ${rest} с` : `${minutes} мин`;
};

// Единый порядок вывода статусов матчей/программ; неизвестные ключи — в конец по алфавиту
const STATUS_ORDER = ['pending', 'compiling', 'ready', 'running', 'completed', 'failed', 'cancelled'];

const sortStatusEntries = (record: Record<string, number>): [string, number][] =>
  Object.entries(record).sort(([a], [b]) => {
    const ia = STATUS_ORDER.indexOf(a);
    const ib = STATUS_ORDER.indexOf(b);
    if (ia === -1 && ib === -1) return a.localeCompare(b);
    if (ia === -1) return 1;
    if (ib === -1) return -1;
    return ia - ib;
  });

// Метки и бейджи статусов матчей (matches.by_status); неизвестный ключ выводится как есть, серым
const matchStatusMeta: Record<string, { label: string; badge: string }> = {
  pending: { label: 'Ожидают', badge: 'bg-yellow-900/30 text-yellow-400' },
  running: { label: 'Выполняются', badge: 'bg-blue-900/30 text-blue-400' },
  completed: { label: 'Завершены', badge: 'bg-green-900/30 text-green-400' },
  failed: { label: 'С ошибкой', badge: 'bg-red-900/30 text-red-400' },
  cancelled: { label: 'Отменены', badge: 'bg-gray-700 text-gray-300' },
};

// Метки и бейджи статусов программ (programs)
const programStatusMeta: Record<string, { label: string; badge: string }> = {
  pending: { label: 'Ожидают', badge: 'bg-gray-700 text-gray-300' },
  compiling: { label: 'Компилируются', badge: 'bg-yellow-900/30 text-yellow-400' },
  ready: { label: 'Готовы', badge: 'bg-green-900/30 text-green-400' },
  failed: { label: 'С ошибкой', badge: 'bg-red-900/30 text-red-400' },
};

const unknownStatusBadge = 'bg-gray-700 text-gray-300';

interface SystemTabProps {
  queueStats: QueueStats | null;
  matchStats: MatchStatistics | null;
  systemMetrics: SystemMetrics | null;
  failedMatches: Match[];
  fullStatus: FullSystemStatus | null;
  fullStatusIsError: boolean;
  isLoadingSystem: boolean;
  showLoadingSystem: boolean;
  allSystemQueriesFailed: boolean;
  systemError: string | null;
  setSystemError: Dispatch<SetStateAction<string | null>>;
  isClearing: boolean;
  setIsClearing: Dispatch<SetStateAction<boolean>>;
  isPurging: boolean;
  setIsPurging: Dispatch<SetStateAction<boolean>>;
  recoveryBusy: string | null;
  setRecoveryBusy: Dispatch<SetStateAction<string | null>>;
  refreshSystemData: () => void;
  setAdminReaction: AdminReactionSetter;
}

export function SystemTab({
  queueStats,
  matchStats,
  systemMetrics,
  failedMatches,
  fullStatus,
  fullStatusIsError,
  isLoadingSystem,
  showLoadingSystem,
  allSystemQueriesFailed,
  systemError,
  setSystemError,
  isClearing,
  setIsClearing,
  isPurging,
  setIsPurging,
  recoveryBusy,
  setRecoveryBusy,
  refreshSystemData,
  setAdminReaction,
}: SystemTabProps) {
  // Очередь: приоритеты из useQueueStats, фолбэк — раздел queues нового /system/status
  const queueNumbers = queueStats ?? fullStatus?.queues ?? null;
  // Статусы матчей: полный набор ключей из /system/status, фолбэк — агрегаты useMatchStatistics
  const matchStatusEntries: [string, number][] = fullStatus
    ? sortStatusEntries(fullStatus.matches.by_status)
    : matchStats
      ? [
          ['pending', matchStats.pending],
          ['running', matchStats.running],
          ['completed', matchStats.completed],
          ['failed', matchStats.failed],
        ]
      : [];
  const matchesTotal =
    matchStats?.total ?? matchStatusEntries.reduce((sum, [, count]) => sum + count, 0);

  const handleClearQueue = async () => {
    if (!(await confirmDialog({
      title: 'Очистка очереди',
      message: 'Очистить очередь? Все ожидающие матчи будут удалены.',
      confirmLabel: 'Очистить',
      danger: true,
    }))) {
      return;
    }
    setIsClearing(true);
    setSystemError(null);
    try {
      await api.clearQueue();
      refreshSystemData();
      setAdminReaction('dizzy', '// очередь пуста', 2500);
    } catch (err) {
      console.error('Failed to clear queue:', err);
      setSystemError('Не удалось очистить очередь');
    } finally {
      setIsClearing(false);
    }
  };

  const handlePurgeInvalidMatches = async () => {
    setIsPurging(true);
    setSystemError(null);
    try {
      const result = await api.purgeInvalidMatches();
      useToastStore.getState().addToast(`Удалено ${result.purged_count} невалидных матчей из очереди`, 'success');
      refreshSystemData();
    } catch (err) {
      console.error('Failed to purge invalid matches:', err);
      setSystemError('Не удалось очистить невалидные матчи');
    } finally {
      setIsPurging(false);
    }
  };

  // Кнопки восстановления: прикладные поломки чинятся прямо из интерфейса.
  const handleRecovery = async (
    action: 'outbox' | 'compile' | 'stuck' | 'deadletter',
    confirmText: string,
    run: () => Promise<string>
  ) => {
    if (!(await confirmDialog({ title: 'Восстановление', message: confirmText, confirmLabel: 'Выполнить' }))) return;
    setRecoveryBusy(action);
    setSystemError(null);
    try {
      const message = await run();
      useToastStore.getState().addToast(message, 'success', 8000);
      refreshSystemData();
    } catch (err) {
      console.error(`Recovery action ${action} failed:`, err);
      setSystemError('Действие восстановления не удалось — подробности в консоли');
    } finally {
      setRecoveryBusy(null);
    }
  };

  return (
        <div>
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-lg font-semibold text-gray-100">Состояние системы</h2>
            <button
              onClick={refreshSystemData}
              disabled={isLoadingSystem}
              className="btn btn-secondary text-sm"
            >
              {isLoadingSystem ? 'Обновление...' : 'Обновить'}
            </button>
          </div>

          {(systemError || allSystemQueriesFailed) && (
            <div className="mb-4 p-3 bg-red-900/30 border border-red-800 rounded text-sm text-red-400">
              {systemError ?? 'Не удалось загрузить данные системы'}
            </div>
          )}

          {isLoadingSystem && !queueStats && !matchStats && !fullStatus && !showLoadingSystem ? (
            null
          ) : showLoadingSystem && !queueStats && !matchStats && !fullStatus ? (
            <div className="text-center py-8 text-gray-400">
              Загрузка данных системы...
            </div>
          ) : (
            <div className="grid gap-6 md:grid-cols-2">
              {/* Services Health Card */}
              <div className="card md:col-span-2">
                <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">🩺</span>
                  Состояние сервисов
                </h3>
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  {/* API */}
                  <div className="p-3 border border-gray-700 rounded-lg">
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`w-2 h-2 rounded-full ${fullStatus ? 'bg-green-500' : 'bg-red-500'}`} />
                      <span className="text-sm font-medium text-gray-300">API</span>
                    </div>
                    <p className="text-xs text-gray-400">
                      {fullStatus ? 'Работает' : fullStatusIsError ? 'Недоступен' : 'Нет данных'}
                    </p>
                  </div>
                  {/* PostgreSQL */}
                  <div
                    className={`p-3 border rounded-lg ${
                      fullStatus?.database.schema_dirty
                        ? 'border-red-800 bg-red-900/20'
                        : 'border-gray-700'
                    }`}
                  >
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`w-2 h-2 rounded-full ${fullStatus?.database.healthy ? 'bg-green-500' : 'bg-red-500'}`} />
                      <span className="text-sm font-medium text-gray-300">PostgreSQL</span>
                    </div>
                    {fullStatus ? (
                      <p className="text-xs text-gray-400">
                        Миграции: v{fullStatus.database.schema_version}
                        {fullStatus.database.schema_dirty && (
                          <span className="text-red-400 font-medium"> (dirty!)</span>
                        )}
                        <br />
                        Соединения: {fullStatus.database.in_use}/{fullStatus.database.max_open}
                      </p>
                    ) : (
                      <p className="text-xs text-gray-400">Нет данных</p>
                    )}
                  </div>
                  {/* Redis */}
                  <div className="p-3 border border-gray-700 rounded-lg">
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`w-2 h-2 rounded-full ${fullStatus?.redis.healthy ? 'bg-green-500' : 'bg-red-500'}`} />
                      <span className="text-sm font-medium text-gray-300">Redis</span>
                    </div>
                    <p className="text-xs text-gray-400">
                      {fullStatus ? (fullStatus.redis.healthy ? 'Работает' : 'Недоступен') : 'Нет данных'}
                    </p>
                  </div>
                  {/* WebSocket */}
                  <div className="p-3 border border-gray-700 rounded-lg">
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`w-2 h-2 rounded-full ${fullStatus ? 'bg-green-500' : 'bg-red-500'}`} />
                      <span className="text-sm font-medium text-gray-300">WebSocket</span>
                    </div>
                    <p className="text-xs text-gray-400">
                      {fullStatus
                        ? `${fullStatus.websocket.total_clients ?? 0} подключений / ${fullStatus.websocket.tournaments ?? 0} каналов`
                        : 'Нет данных'}
                    </p>
                  </div>
                </div>
              </div>

              {/* Version & Uptime Card */}
              <div className="card">
                <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">📦</span>
                  Версия и аптайм
                </h3>
                {fullStatus ? (
                  <div className="space-y-3 text-sm">
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Версия</span>
                      <span className="font-medium text-gray-100 font-mono">
                        {fullStatus.app.version}
                        {fullStatus.app.dirty && <span className="text-orange-400">-dirty</span>}
                      </span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Go</span>
                      <span className="font-medium text-gray-100 font-mono">{fullStatus.app.go_version}</span>
                    </div>
                    <div className="flex justify-between items-center">
                      <span className="text-gray-400">Аптайм</span>
                      <span className="font-medium text-gray-100">{formatUptime(fullStatus.app.uptime_seconds)}</span>
                    </div>
                    {fullStatus.app.build_time && (
                      <div className="flex justify-between items-center">
                        <span className="text-gray-400">Сборка</span>
                        <span className="font-medium text-gray-100">
                          {Number.isNaN(Date.parse(fullStatus.app.build_time))
                            ? fullStatus.app.build_time
                            : new Date(fullStatus.app.build_time).toLocaleString('ru-RU')}
                        </span>
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-gray-400">Нет данных</p>
                )}
              </div>

              {/* Programs Card */}
              <div className="card">
                <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">📁</span>
                  Программы
                </h3>
                {fullStatus ? (
                  Object.keys(fullStatus.programs).length === 0 ? (
                    <p className="text-gray-400">Программы ещё не загружены</p>
                  ) : (
                    <div className="space-y-3 text-sm">
                      {sortStatusEntries(fullStatus.programs).map(([status, count]) => {
                        const meta = programStatusMeta[status] ?? { label: status, badge: unknownStatusBadge };
                        return (
                          <div key={status} className="flex justify-between items-center">
                            <span className="text-gray-400">{meta.label}</span>
                            <span className={`px-2 py-1 rounded font-medium ${meta.badge}`}>{count}</span>
                          </div>
                        );
                      })}
                    </div>
                  )
                ) : (
                  <p className="text-gray-400">Нет данных</p>
                )}
              </div>

              {/* Queue Stats Card */}
              <div className="card">
                <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">📊</span>
                  Очередь матчей
                </h3>
                {queueNumbers ? (
                  <div className="space-y-3">
                    <div className="flex justify-between items-center py-2 border-b border-gray-700">
                      <span className="text-gray-400">Всего в очереди</span>
                      <span className="text-2xl font-bold text-gray-100">{queueNumbers.total}</span>
                    </div>
                    <div className="grid grid-cols-3 gap-3 pt-2">
                      <div className="text-center">
                        <div className="text-xs text-gray-400 mb-1">Высокий</div>
                        <div className="text-lg font-semibold text-red-400">{queueNumbers.high}</div>
                      </div>
                      <div className="text-center">
                        <div className="text-xs text-gray-400 mb-1">Средний</div>
                        <div className="text-lg font-semibold text-yellow-400">{queueNumbers.medium}</div>
                      </div>
                      <div className="text-center">
                        <div className="text-xs text-gray-400 mb-1">Низкий</div>
                        <div className="text-lg font-semibold text-blue-400">{queueNumbers.low}</div>
                      </div>
                    </div>
                    {fullStatus && (
                      <div className="flex flex-wrap gap-2 pt-3 border-t border-gray-700">
                        <span
                          className={`px-2 py-1 text-xs rounded font-medium ${
                            fullStatus.queues.dead_letter > 0
                              ? 'bg-red-900/30 text-red-400'
                              : 'bg-gray-700 text-gray-300'
                          }`}
                        >
                          Dead letter: {fullStatus.queues.dead_letter}
                        </span>
                        <span
                          className={`px-2 py-1 text-xs rounded font-medium ${
                            fullStatus.queues.compile > 0
                              ? 'bg-yellow-900/30 text-yellow-400'
                              : 'bg-gray-700 text-gray-300'
                          }`}
                        >
                          Компиляция: {fullStatus.queues.compile}
                        </span>
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-gray-400">Нет данных</p>
                )}
              </div>

              {/* Match Stats Card: статусы из /system/status (все ключи by_status), фолбэк — useMatchStatistics */}
              <div className="card">
                <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">🎮</span>
                  Статистика матчей
                </h3>
                {matchStats || fullStatus ? (
                  <div className="space-y-3">
                    <div className="flex justify-between items-center py-2 border-b border-gray-700">
                      <span className="text-gray-400">Всего матчей</span>
                      <span className="text-2xl font-bold text-gray-100">{matchesTotal}</span>
                    </div>
                    <div className="grid grid-cols-2 gap-3 pt-2">
                      {matchStatusEntries.map(([status, count]) => {
                        const meta = matchStatusMeta[status] ?? { label: status, badge: unknownStatusBadge };
                        return (
                          <div key={status} className="flex justify-between items-center">
                            <span className="text-gray-400">{meta.label}</span>
                            <span className={`px-2 py-1 rounded font-medium ${meta.badge}`}>{count}</span>
                          </div>
                        );
                      })}
                    </div>
                    {fullStatus?.matches.last_completed_at && (
                      <p className="text-xs text-gray-400 pt-2 border-t border-gray-700">
                        Последний завершённый:{' '}
                        {new Date(fullStatus.matches.last_completed_at).toLocaleString('ru-RU')}
                      </p>
                    )}
                  </div>
                ) : (
                  <p className="text-gray-400">Нет данных</p>
                )}
              </div>

              {/* Outbox Card */}
              <div className="card md:col-span-2">
                <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">✉️</span>
                  Outbox (целостность рейтингов)
                </h3>
                {fullStatus?.outbox ? (
                  // flex + justify-evenly вместо grid-cols-4: четвёртый элемент
                  // условный, и с фиксированной сеткой три элемента прижимались
                  // влево, оставляя пустую правую колонку.
                  <div className="flex flex-wrap justify-evenly gap-3">
                    <div className="text-center min-w-[110px]">
                      <div className="text-xs text-gray-400 mb-1">Ожидают</div>
                      <div className="text-lg font-semibold text-gray-100">{fullStatus.outbox.pending}</div>
                    </div>
                    <div className="text-center min-w-[110px]">
                      <div className="text-xs text-gray-400 mb-1">Ошибки</div>
                      <div
                        className={`text-lg font-semibold ${
                          fullStatus.outbox.errors > 0 ? 'text-red-400' : 'text-gray-100'
                        }`}
                      >
                        {fullStatus.outbox.errors}
                      </div>
                    </div>
                    <div className="text-center min-w-[110px]">
                      <div className="text-xs text-gray-400 mb-1">Выполнено за 24ч</div>
                      <div className="text-lg font-semibold text-green-400">{fullStatus.outbox.done_last_24h}</div>
                    </div>
                    {typeof fullStatus.outbox.oldest_pending_age_seconds === 'number' && (
                      <div className="text-center min-w-[110px]">
                        <div className="text-xs text-gray-400 mb-1">Старейшая ожидающая</div>
                        <div className="text-lg font-semibold text-gray-100">
                          {formatAgeSeconds(fullStatus.outbox.oldest_pending_age_seconds)}
                        </div>
                      </div>
                    )}
                  </div>
                ) : (
                  <p className="text-gray-400">Нет данных</p>
                )}
                <p className="text-xs text-gray-400 mt-3">
                  Задачи пост-обработки матчей; errors &gt; 0 требует внимания.
                </p>
              </div>

              {/* Recovery Card: починка прикладных поломок из интерфейса */}
              <div className="card md:col-span-2">
                <h3 className="text-lg font-semibold text-gray-100 mb-4">
                  Восстановление
                </h3>
                <div className="flex flex-wrap gap-3">
                  <button
                    onClick={() =>
                      handleRecovery(
                        'stuck',
                        'Сбросить зависшие матчи (running > 2 минут) в очередь на повторное выполнение?',
                        async () => {
                          const r = await api.recoveryResetStuckMatches();
                          return `Возвращено в очередь: ${r.reset} матчей`;
                        }
                      )
                    }
                    disabled={recoveryBusy !== null}
                    className={`btn ${
                      (fullStatus?.matches?.stuck_running ?? 0) > 0 ? 'btn-danger' : 'btn-secondary'
                    }`}
                  >
                    {recoveryBusy === 'stuck'
                      ? 'Сброс...'
                      : `Сбросить зависшие матчи${
                          (fullStatus?.matches?.stuck_running ?? 0) > 0
                            ? ` (${fullStatus?.matches?.stuck_running})`
                            : ''
                        }`}
                  </button>
                  <button
                    onClick={() =>
                      handleRecovery(
                        'outbox',
                        'Повторить ошибочные outbox-задачи (рейтинги, которые не применились)?',
                        async () => {
                          const r = await api.recoveryRetryOutbox();
                          return `Возвращено в обработку: ${r.retried} задач`;
                        }
                      )
                    }
                    disabled={recoveryBusy !== null}
                    className={`btn ${
                      (fullStatus?.outbox?.errors ?? 0) > 0 ? 'btn-danger' : 'btn-secondary'
                    }`}
                  >
                    {recoveryBusy === 'outbox'
                      ? 'Повтор...'
                      : `Повторить outbox-ошибки${
                          (fullStatus?.outbox?.errors ?? 0) > 0 ? ` (${fullStatus?.outbox?.errors})` : ''
                        }`}
                  </button>
                  <button
                    onClick={() =>
                      handleRecovery(
                        'compile',
                        'Перезапустить компиляцию всех программ в статусе compiling?',
                        async () => {
                          const r = await api.recoveryRequeueCompiling();
                          return `Поставлено в очередь компиляции: ${r.requeued} программ`;
                        }
                      )
                    }
                    disabled={recoveryBusy !== null}
                    className={`btn ${
                      (fullStatus?.programs?.compiling ?? 0) > 0 ? 'btn-warning' : 'btn-secondary'
                    }`}
                  >
                    {recoveryBusy === 'compile'
                      ? 'Перезапуск...'
                      : `Перезапустить компиляцию${
                          (fullStatus?.programs?.compiling ?? 0) > 0
                            ? ` (${fullStatus?.programs?.compiling})`
                            : ''
                        }`}
                  </button>
                  <button
                    onClick={() =>
                      handleRecovery(
                        'deadletter',
                        'Очистить dead-letter очередь? Повреждённые задачи будут удалены безвозвратно.',
                        async () => {
                          const r = await api.recoveryClearDeadLetter();
                          return `Удалено из dead-letter: ${r.cleared} записей`;
                        }
                      )
                    }
                    disabled={recoveryBusy !== null}
                    className={`btn ${
                      (fullStatus?.queues?.dead_letter ?? 0) > 0 ? 'btn-danger' : 'btn-secondary'
                    }`}
                  >
                    {recoveryBusy === 'deadletter'
                      ? 'Очистка...'
                      : `Очистить dead-letter${
                          (fullStatus?.queues?.dead_letter ?? 0) > 0
                            ? ` (${fullStatus?.queues?.dead_letter})`
                            : ''
                        }`}
                  </button>
                </div>
                <p className="text-xs text-gray-400 mt-3">
                  Кнопки подсвечиваются, когда есть что чинить. Зависшие матчи и компиляция
                  перезапускаются безопасно (идемпотентно); очистка dead-letter необратима.
                </p>
              </div>

              {/* System Metrics Card */}
              <div className="card md:col-span-2">
                <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">💻</span>
                  Нагрузка сервера
                </h3>
                {systemMetrics ? (
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                    {/* CPU */}
                    <div className="space-y-3">
                      <div className="flex items-center gap-2 text-sm font-medium text-gray-300">
                        <span>🔧</span> CPU
                      </div>
                      <div className="relative pt-1">
                        <div className="flex mb-2 items-center justify-between">
                          <span className="text-xs font-semibold inline-block text-gray-400">
                            {systemMetrics.cpu.usage_percent.toFixed(1)}%
                          </span>
                          <span className="text-xs text-gray-400">
                            {systemMetrics.cpu.cores} ядер
                          </span>
                        </div>
                        <div className="overflow-hidden h-2 text-xs flex rounded bg-gray-700">
                          <div
                            style={{ width: `${Math.min(systemMetrics.cpu.usage_percent, 100)}%` }}
                            className={`shadow-none flex flex-col text-center whitespace-nowrap text-white justify-center transition-[width] duration-300 ${
                              systemMetrics.cpu.usage_percent > 80
                                ? 'bg-red-500'
                                : systemMetrics.cpu.usage_percent > 50
                                ? 'bg-yellow-500'
                                : 'bg-green-500'
                            }`}
                          />
                        </div>
                      </div>
                      {systemMetrics.cpu.model_name && (
                        <p className="text-xs text-gray-400 truncate" title={systemMetrics.cpu.model_name}>
                          {systemMetrics.cpu.model_name}
                        </p>
                      )}
                    </div>

                    {/* Memory */}
                    <div className="space-y-3">
                      <div className="flex items-center gap-2 text-sm font-medium text-gray-300">
                        <span>🧠</span> Память
                      </div>
                      <div className="relative pt-1">
                        <div className="flex mb-2 items-center justify-between">
                          <span className="text-xs font-semibold inline-block text-gray-400">
                            {systemMetrics.memory.used_percent.toFixed(1)}%
                          </span>
                          <span className="text-xs text-gray-400">
                            {formatBytes(systemMetrics.memory.used)} / {formatBytes(systemMetrics.memory.total)}
                          </span>
                        </div>
                        <div className="overflow-hidden h-2 text-xs flex rounded bg-gray-700">
                          <div
                            style={{ width: `${Math.min(systemMetrics.memory.used_percent, 100)}%` }}
                            className={`shadow-none flex flex-col text-center whitespace-nowrap text-white justify-center transition-[width] duration-300 ${
                              systemMetrics.memory.used_percent > 80
                                ? 'bg-red-500'
                                : systemMetrics.memory.used_percent > 50
                                ? 'bg-yellow-500'
                                : 'bg-green-500'
                            }`}
                          />
                        </div>
                      </div>
                      <p className="text-xs text-gray-400">
                        Свободно: {formatBytes(systemMetrics.memory.free)}
                      </p>
                    </div>

                    {/* Disk */}
                    <div className="space-y-3">
                      <div className="flex items-center gap-2 text-sm font-medium text-gray-300">
                        <span>💾</span> Диск ({systemMetrics.disk.path})
                      </div>
                      <div className="relative pt-1">
                        <div className="flex mb-2 items-center justify-between">
                          <span className="text-xs font-semibold inline-block text-gray-400">
                            {systemMetrics.disk.used_percent.toFixed(1)}%
                          </span>
                          <span className="text-xs text-gray-400">
                            {formatBytes(systemMetrics.disk.used)} / {formatBytes(systemMetrics.disk.total)}
                          </span>
                        </div>
                        <div className="overflow-hidden h-2 text-xs flex rounded bg-gray-700">
                          <div
                            style={{ width: `${Math.min(systemMetrics.disk.used_percent, 100)}%` }}
                            className={`shadow-none flex flex-col text-center whitespace-nowrap text-white justify-center transition-[width] duration-300 ${
                              systemMetrics.disk.used_percent > 90
                                ? 'bg-red-500'
                                : systemMetrics.disk.used_percent > 70
                                ? 'bg-yellow-500'
                                : 'bg-green-500'
                            }`}
                          />
                        </div>
                      </div>
                      <p className="text-xs text-gray-400">
                        Свободно: {formatBytes(systemMetrics.disk.free)}
                      </p>
                    </div>
                  </div>
                ) : (
                  <p className="text-gray-400">Нет данных</p>
                )}

                {/* Temperature sensors */}
                {systemMetrics && (
                  <div className="mt-6 pt-4 border-t border-gray-700">
                    <div className="flex items-center gap-2 text-sm font-medium text-gray-300 mb-3">
                      <span>🌡️</span> Температура
                    </div>
                    {systemMetrics.temperature && systemMetrics.temperature.length > 0 ? (
                      <div className="flex flex-wrap gap-3">
                        {systemMetrics.temperature.map((temp, idx) => (
                          <div
                            key={idx}
                            className={`px-3 py-2 rounded-lg text-sm ${
                              temp.temperature > 80
                                ? 'bg-red-900/30 text-red-400'
                                : temp.temperature > 60
                                ? 'bg-yellow-900/30 text-yellow-400'
                                : 'bg-green-900/30 text-green-400'
                            }`}
                          >
                            <span className="font-medium">{temp.temperature.toFixed(1)}°C</span>
                            <span className="text-xs opacity-75 ml-1">{temp.sensor_key}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-sm text-gray-400">
                        Датчики температуры недоступны на этой системе (macOS не поддерживает)
                      </p>
                    )}
                  </div>
                )}

                {/* Go runtime info */}
                {systemMetrics && (
                  <div className="mt-6 pt-4 border-t border-gray-700">
                    <div className="flex items-center gap-2 text-sm font-medium text-gray-300 mb-3">
                      <span>🐹</span> Go Runtime
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                      <div>
                        <span className="text-gray-400">Версия:</span>
                        <span className="ml-2 font-medium text-gray-100">{systemMetrics.go.version}</span>
                      </div>
                      <div>
                        <span className="text-gray-400">Горутины:</span>
                        <span className="ml-2 font-medium text-gray-100">{systemMetrics.go.goroutines}</span>
                      </div>
                      <div>
                        <span className="text-gray-400">Heap:</span>
                        <span className="ml-2 font-medium text-gray-100">{formatBytes(systemMetrics.go.heap_alloc)}</span>
                      </div>
                      <div>
                        <span className="text-gray-400">GC:</span>
                        <span className="ml-2 font-medium text-gray-100">{systemMetrics.go.num_gc} циклов</span>
                      </div>
                    </div>
                  </div>
                )}

                {/* Host info */}
                {systemMetrics && (
                  <div className="mt-4 pt-4 border-t border-gray-700">
                    <div className="flex items-center gap-2 text-sm font-medium text-gray-300 mb-3">
                      <span>🖥️</span> Система
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-sm">
                      <div>
                        <span className="text-gray-400">Хост:</span>
                        <span className="ml-2 font-medium text-gray-100">{systemMetrics.host.hostname}</span>
                      </div>
                      <div>
                        <span className="text-gray-400">ОС:</span>
                        <span className="ml-2 font-medium text-gray-100">{systemMetrics.host.platform} {systemMetrics.host.platform_version}</span>
                      </div>
                      <div>
                        <span className="text-gray-400">Архитектура:</span>
                        <span className="ml-2 font-medium text-gray-100">{systemMetrics.host.arch}</span>
                      </div>
                      <div>
                        <span className="text-gray-400">Uptime:</span>
                        <span className="ml-2 font-medium text-gray-100">{formatUptime(systemMetrics.host.uptime)}</span>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Failed Matches Card */}
              {failedMatches.length > 0 && (
                <div className="card md:col-span-2">
                  <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                    <span className="text-xl">⚠️</span>
                    Провалившиеся матчи ({failedMatches.length})
                  </h3>
                  <div className="space-y-3 max-h-64 overflow-y-auto">
                    {failedMatches.map((match) => (
                      <div
                        key={match.id}
                        className="p-3 bg-red-900/20 border border-red-800 rounded-lg"
                      >
                        <div className="flex justify-between items-start mb-2">
                          <div className="text-sm font-medium text-gray-100">
                            Матч {match.id.substring(0, 8)}...
                          </div>
                          <span className="text-xs text-gray-400">
                            {match.game_type}
                          </span>
                        </div>
                        {match.error_message && (
                          <div className="text-sm text-red-400 font-mono bg-red-900/30 p-2 rounded text-xs whitespace-pre-wrap break-words">
                            {match.error_message}
                          </div>
                        )}
                        <div className="mt-2 text-xs text-gray-400">
                          Код ошибки: {match.error_code || 'N/A'}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Queue Actions Card */}
              <div className="card md:col-span-2">
                <h3 className="text-md font-semibold text-gray-100 mb-4 flex items-center gap-2">
                  <span className="text-xl">🛠</span>
                  Управление очередью
                </h3>
                <div className="flex flex-wrap gap-3">
                  <button
                    onClick={handlePurgeInvalidMatches}
                    disabled={isPurging}
                    className="btn btn-secondary"
                  >
                    {isPurging ? 'Очистка...' : 'Удалить невалидные матчи'}
                  </button>
                  <button
                    onClick={handleClearQueue}
                    disabled={isClearing}
                    className="btn btn-danger"
                  >
                    {isClearing ? 'Очистка...' : 'Очистить всю очередь'}
                  </button>
                </div>
                <p className="text-xs text-gray-400 mt-3">
                  «Удалить невалидные матчи» — удаляет из очереди матчи, которые не существуют в базе данных.
                  «Очистить всю очередь» — удаляет все матчи из очереди (требует подтверждения).
                </p>
              </div>
            </div>
          )}
        </div>
  );
}
