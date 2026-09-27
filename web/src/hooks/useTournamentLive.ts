// Живые обновления турнира: WebSocket-события → точечная инвалидация
// кэша TanStack Query.
//
// Поллинг включается как fallback, когда WS-соединения нет (pollInterval из
// этого хука), и редкий - пока идёт раунд: упавшие и отменённые матчи событий
// не шлют, и конец игры, чьи последние матчи упали, иначе не был бы виден. Это касается и анонимов: /ws требует токен,
// поэтому у них живых событий нет и данные обновляет поллинг. У не идущего
// турнира матчи не меняются, и поллинга нет. Статус турнира без WS
// перечитывается редко: иначе открытая до старта вкладка так и не узнала бы,
// что турнир пошёл, а после завершения продолжала бы поллинг.

import { useCallback, useEffect, useMemo, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useWebSocket } from './useWebSocket';
import { parseTournamentWSMessage } from '../types/ws';
import type { MatchResultPayload } from '../types/ws';
import type { WSMessage, Program } from '../types';
import { queryKeys } from '../api/queryKeys';
import { FALLBACK_POLL_INTERVAL, useMatchesByRounds, useTournament } from './queries';

interface UseTournamentLiveOptions {
  tournamentId: string;
  enabled?: boolean;
  /** Каждый результат матча сразу, без throttle: лента табло. */
  onMatchResult?: (payload: MatchResultPayload) => void;
}

export const TOURNAMENT_STATUS_POLL_INTERVAL = 30_000;
export const ROUND_POLL_INTERVAL = 15_000;

// Во время раунда match_result идут непрерывно (по событию на матч), а каждая
// инвалидация - это запросы лидерборда и раундов. Окно 5с держит вкладку
// в пределах серверного rate limit.
const INVALIDATE_THROTTLE_MS = 5000;

/**
 * Throttle с первым и хвостовым вызовом: первое событие обновляет данные сразу,
 * всё, что пришло внутри окна, схлопывается в один вызов в конце окна.
 */
export function throttle(fn: () => void, ms: number) {
  let timer: ReturnType<typeof setTimeout> | null = null;
  let pending = false;
  const openWindow = () => {
    timer = setTimeout(() => {
      timer = null;
      if (pending) {
        pending = false;
        fn();
        openWindow();
      }
    }, ms);
  };
  const call = () => {
    if (timer) {
      pending = true;
      return;
    }
    fn();
    openWindow();
  };
  call.cancel = () => {
    if (timer) clearTimeout(timer);
    timer = null;
    pending = false;
  };
  return call;
}

export function useTournamentLive({ tournamentId, enabled = true, onMatchResult }: UseTournamentLiveOptions) {
  const queryClient = useQueryClient();
  const onMatchResultRef = useRef(onMatchResult);
  useEffect(() => {
    onMatchResultRef.current = onMatchResult;
  });

  const scheduleMatchInvalidation = useMemo(
    () =>
      throttle(() => {
        // Идущий запрос отменяется и уходит заново: его ответ мог не застать
        // события окна. Запросы этих ключей передают signal в axios, так что
        // отмена обрывает HTTP и запросы на сервере не копятся.
        void queryClient.invalidateQueries({ queryKey: queryKeys.crossGameLeaderboard(tournamentId) });
        // Счётчики раундов и открытые страницы матчей (ключи вложены).
        void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
        // Ключи страницы игры: лидерборд и матчи. Head-to-head - тяжёлая матрица
        // по всем матчам игры, она обновляется по staleTime и фокусу.
        void queryClient.invalidateQueries({
          queryKey: ['tournament', tournamentId, 'game'],
          predicate: (q) => q.queryKey[4] !== 'head-to-head',
        });
        // Авто-раунд сдвигает last_run_at, отдельного события нет.
        void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
      }, INVALIDATE_THROTTLE_MS),
    [queryClient, tournamentId]
  );
  useEffect(() => scheduleMatchInvalidation.cancel, [scheduleMatchInvalidation]);

  const handleMessage = useCallback(
    (raw: WSMessage) => {
      const message = parseTournamentWSMessage(raw);
      if (!message) return;

      switch (message.type) {
        case 'tournament_update':
          // Статус турнира меняется редко - инвалидация сразу, без throttle.
          void queryClient.invalidateQueries({ queryKey: queryKeys.tournament(tournamentId) });
          break;

        case 'match_result':
          // Рейтинги уже в payload, но позиции лидерборда и тайбрейки
          // считает сервер - редкая инвалидация дешевле и корректнее
          // ручного патча сортировки.
          scheduleMatchInvalidation();
          onMatchResultRef.current?.(message.payload);
          break;

        case 'program_update': {
          // Статус компиляции патчится в кэш сразу. Текста ошибки в WS нет
          // (рассылка идёт всему турниру), а свежей версии сокомандника в кэше
          // ещё нет, поэтому программы перечитываются всегда: события редкие.
          const { program_id, status, check_status } = message.payload;
          queryClient.setQueriesData<Program[]>(
            { queryKey: queryKeys.programs },
            (old) => old?.map((p) => (p.id === program_id ? { ...p, status, check_status: check_status ?? p.check_status } : p))
          );
          // ['programs'] - префикс и списка, и версий.
          void queryClient.invalidateQueries({ queryKey: queryKeys.programs });
          break;
        }
      }
    },
    [queryClient, tournamentId, scheduleMatchInvalidation]
  );

  const { isConnected, isOnline, reconnect } = useWebSocket({
    tournamentId,
    enabled,
    onMessage: handleMessage,
  });

  // Тот же ключ, что у страницы: TanStack сводит оба observer'а в один запрос.
  const tournament = useTournament(tournamentId, {
    pollInterval: isConnected ? false : TOURNAMENT_STATUS_POLL_INTERVAL,
  });
  const active = tournament.data?.status === 'active';
  // Только чтение кэша раундов: запрос ведёт страница, где они нужны.
  const rounds = useMatchesByRounds(tournamentId, { enabled: false }).data;
  const roundRunning = !!rounds?.some((r) => r.pending_count + r.running_count > 0);

  const pollInterval: number | false = !active
    ? false
    : !isConnected
      ? FALLBACK_POLL_INTERVAL
      : roundRunning
        ? ROUND_POLL_INTERVAL
        : false;

  return { isConnected, isOnline, reconnect, pollInterval };
}
