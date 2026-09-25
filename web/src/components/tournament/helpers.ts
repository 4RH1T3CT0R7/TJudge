import axios from 'axios';
import type { QueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import type { TournamentStatus } from '../../types';

export function extractErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    return err.response?.data?.error || err.response?.data?.message || fallback;
  }
  return err instanceof Error ? err.message : fallback;
}

export const statusConfig: Record<TournamentStatus, {
  badge: string;
  label: string;
}> = {
  pending: {
    badge: 'badge badge-yellow',
    label: 'Ожидание',
  },
  active: {
    badge: 'badge badge-green',
    label: 'Активный',
  },
  completed: {
    badge: 'badge badge-gray',
    label: 'Завершён',
  },
};

// Ждёт, пока матчи турнира доиграют, и обновляет лидерборд с раундами по ходу.
// Упавшие матчи автоматически не перезапускаются: ошибка программы
// детерминирована, и авто-ретрай из браузера гонял бы её матчи по кругу.
// Для ретраев есть кнопка «Перезапустить неудачные» и recovery на бэкенде.
export async function waitForMatchesAndAutoRetry(
  queryClient: QueryClient,
  targetTournamentId: string,
  initialEnqueued: number
) {
  const MAX_WAIT_TIME = 10 * 60 * 1000; // 10 minutes max
  // Не чаще живых инвалидаций (useTournamentLive): вкладка админа тоже под rate limit
  const POLL_INTERVAL = 5000;

  const startTime = Date.now();
  let lastPending = initialEnqueued;

  while (Date.now() - startTime < MAX_WAIT_TIME) {
    await new Promise(resolve => setTimeout(resolve, POLL_INTERVAL));

    try {
      const stats = await api.getMatchStatistics(targetTournamentId);
      const inProgress = stats.pending + stats.running;

      // Refresh leaderboard while matches are running
      if (inProgress !== lastPending) {
        lastPending = inProgress;
        void queryClient.invalidateQueries({ queryKey: queryKeys.crossGameLeaderboard(targetTournamentId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.matchesByRounds(targetTournamentId) });
      }

      // All matches completed
      if (inProgress === 0) return;
    } catch (err) {
      console.error('Error polling match status:', err);
    }
  }

  console.warn('Timeout waiting for matches to complete');
}
