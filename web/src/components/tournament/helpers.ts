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

// Helper function to wait for matches to complete and auto-retry if needed
export async function waitForMatchesAndAutoRetry(
  queryClient: QueryClient,
  targetTournamentId: string,
  initialEnqueued: number
) {
  const MAX_WAIT_TIME = 10 * 60 * 1000; // 10 minutes max
  const POLL_INTERVAL = 2000; // 2 seconds
  const AUTO_RETRY_THRESHOLD = 50;

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
      if (inProgress === 0) {
        // Check for failed matches
        if (stats.failed > 0 && stats.failed <= AUTO_RETRY_THRESHOLD) {
          console.log(`Auto-retrying ${stats.failed} failed matches (threshold: ${AUTO_RETRY_THRESHOLD})`);
          try {
            const retryResult = await api.retryFailedMatches(targetTournamentId);
            if (retryResult.enqueued > 0) {
              // Wait for retry to complete recursively
              await waitForMatchesAndAutoRetry(queryClient, targetTournamentId, retryResult.enqueued);
            }
          } catch (retryErr) {
            console.error('Failed to auto-retry matches:', retryErr);
          }
        }
        return;
      }
    } catch (err) {
      console.error('Error polling match status:', err);
    }
  }

  console.warn('Timeout waiting for matches to complete');
}
