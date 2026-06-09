// Живые обновления турнира: WebSocket-события → точечная инвалидация
// кэша TanStack Query.
//
// Раньше WS-сообщения работали как «пинг»: payload игнорировался, на каждое
// событие шёл полный REST-рефетч, и параллельно крутился поллинг каждые 2с.
// Теперь payload используется по назначению, а поллинг включается ТОЛЬКО
// как fallback, когда WS-соединение недоступно (pollInterval из этого хука).

import { useCallback, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useWebSocket } from './useWebSocket';
import { parseTournamentWSMessage } from '../types/ws';
import type { WSMessage, Program } from '../types';
import { queryKeys } from '../api/queryKeys';
import { FALLBACK_POLL_INTERVAL } from './queries';

interface UseTournamentLiveOptions {
  tournamentId: string;
  enabled?: boolean;
}

// Debounce инвалидаций матчей/лидербордов: при пачке завершившихся матчей
// (раунд на 100 пар) не нужно дёргать рефетч на каждое событие.
const INVALIDATE_DEBOUNCE_MS = 500;

export function useTournamentLive({ tournamentId, enabled = true }: UseTournamentLiveOptions) {
  const queryClient = useQueryClient();
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingMatchInvalidate = useRef(false);

  const flushMatchInvalidations = useCallback(() => {
    if (!pendingMatchInvalidate.current) return;
    pendingMatchInvalidate.current = false;

    void queryClient.invalidateQueries({ queryKey: queryKeys.leaderboard(tournamentId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.crossGameLeaderboard(tournamentId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentMatches(tournamentId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(tournamentId) });
    void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(tournamentId) });
  }, [queryClient, tournamentId]);

  const scheduleMatchInvalidation = useCallback(() => {
    pendingMatchInvalidate.current = true;
    if (debounceRef.current) return;
    debounceRef.current = setTimeout(() => {
      debounceRef.current = null;
      flushMatchInvalidations();
    }, INVALIDATE_DEBOUNCE_MS);
  }, [flushMatchInvalidations]);

  const handleMessage = useCallback(
    (raw: WSMessage) => {
      const message = parseTournamentWSMessage(raw);
      if (!message) return;

      switch (message.type) {
        case 'tournament_update':
          // Статус турнира меняется редко - инвалидируем сразу, без debounce.
          void queryClient.invalidateQueries({ queryKey: queryKeys.tournament(tournamentId) });
          break;

        case 'matches_created':
          scheduleMatchInvalidation();
          break;

        case 'match_result':
          // Рейтинги уже в payload, но позиции лидерборда и тайбрейки
          // считает сервер - debounced-инвалидация дешевле и корректнее
          // ручного патча сортировки.
          scheduleMatchInvalidation();
          break;

        case 'program_update': {
          // Статус компиляции патчим в кэш напрямую: payload самодостаточен.
          const { program_id, status, error_message } = message.payload;
          queryClient.setQueriesData<Program[]>(
            { queryKey: queryKeys.programs },
            (old) => old?.map((p) => (p.id === program_id ? { ...p, status, error_message: error_message ?? undefined } : p))
          );
          queryClient.setQueryData<Program>(queryKeys.program(program_id), (old) =>
            old ? { ...old, status, error_message: error_message ?? undefined } : old
          );
          // Списки версий и программ игры (ключи параметризованы) - инвалидация поддерева.
          void queryClient.invalidateQueries({ queryKey: ['programs', 'versions'] });
          void queryClient.invalidateQueries({ queryKey: queryKeys.tournament(tournamentId), exact: false, predicate: (q) => q.queryKey.includes('programs') });
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

  // Fallback-поллинг: только когда живых обновлений нет.
  const pollInterval: number | false =
    enabled && !isConnected ? FALLBACK_POLL_INTERVAL : false;

  return { isConnected, isOnline, reconnect, pollInterval };
}
