import axios from 'axios';
import type { QueryClient } from '@tanstack/react-query';
import api from '../../api/client';
import { queryKeys } from '../../api/queryKeys';
import type { Team } from '../../types';

export function extractErrorMessage(err: unknown, fallback: string): string {
  if (axios.isAxiosError(err)) {
    return err.response?.data?.error || err.response?.data?.message || fallback;
  }
  return err instanceof Error ? err.message : fallback;
}

// Команды, в которые можно вступить из списка: не дисквалифицированы и не
// заполнены (max_team_size 0 - без лимита). Окончательно решает сервер.
export function joinableTeams(teams: Team[], maxTeamSize: number): Team[] {
  return teams.filter((t) => !t.is_disqualified && (maxTeamSize <= 0 || (t.member_count ?? 0) < maxTeamSize));
}

// Текст ошибки вступления из списка: сервер отвечает по-английски
export function joinErrorMessage(err: unknown): string {
  if (!axios.isAxiosError(err)) return 'Не удалось вступить, попробуйте ещё раз';
  const raw = String(err.response?.data?.error ?? '');
  if (raw.includes('full')) return 'Команда уже заполнена';
  if (err.response?.status === 409) return 'Вы уже состоите в команде этого турнира';
  if (err.response?.status === 400) return 'Турнир уже начался: вступить нельзя';
  if (err.response?.status === 404) return 'Команда не найдена: возможно, её удалили';
  return 'Не удалось вступить, попробуйте ещё раз';
}

// Вид таблицы рейтинга: по играм или только сумма.
export const LEADERBOARD_VIEWS = [
  { value: 'games', label: 'По играм' },
  { value: 'total', label: 'Общий' },
] as const;

// Ждёт, пока матчи турнира доиграют, и обновляет лидерборд с раундами по ходу.
// Упавшие матчи автоматически не перезапускаются: ошибка программы
// детерминирована, и авто-ретрай из браузера гонял бы её матчи по кругу.
// Для ретраев есть кнопка «Перезапустить неудачные» и recovery на бэкенде.
export async function waitForMatches(
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
