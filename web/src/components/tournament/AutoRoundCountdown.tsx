import { useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ClockIcon } from '../icons';
import { queryKeys } from '../../api/queryKeys';
import type { TournamentGameWithDetails } from '../../types';

interface AutoRoundCountdownProps {
  status: TournamentGameWithDetails;
  tournamentActive: boolean;
}

// планировщик проверяет игры раз в 5 с: после этого причина в /games/status свежая
const RECHECK_MS = 6000;

// Причины, по которым авто-раунд не стартует, хотя интервал прошёл (см. autoround.go)
const WAIT_TEXT: Record<string, string> = {
  matches_running: 'авто: следующий раунд после текущего',
  new_programs: 'авто: ждёт новую версию программы',
  participants: 'авто: нужно две готовые программы',
};

// Таймер до следующего авто-раунда: мотивирует успеть загрузить новую
// версию программы. Когда отсчёт кончился, а раунд не пошёл, показывает
// причину с последней проверки планировщика, а не «вот-вот».
export function AutoRoundCountdown({ status, tournamentActive }: AutoRoundCountdownProps) {
  const queryClient = useQueryClient();
  const [now, setNow] = useState(() => Date.now());
  const enabled = status.auto_round_enabled && tournamentActive && status.auto_round_interval_seconds > 0;

  useEffect(() => {
    if (!enabled) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [enabled]);

  // Без last_run планировщик стартует от включения — точного времени нет.
  const lastRunAt = status.auto_round_last_run_at;
  const nextAt = lastRunAt ? new Date(lastRunAt).getTime() + status.auto_round_interval_seconds * 1000 : null;
  const msLeft = nextAt !== null ? nextAt - now : null;
  const waitText = status.auto_round_wait ? WAIT_TEXT[status.auto_round_wait] : undefined;
  // статус перечитывается, пока планировщик не ответит (отсчёт кончился, причины нет)
  // и пока идёт раунд: последний match_result мог перечитать статус раньше, чем
  // планировщик сменил matches_running, а других событий может не быть
  const awaitingCheck =
    enabled && (status.auto_round_wait === 'matches_running' || (!waitText && msLeft !== null && msLeft <= 0));

  useEffect(() => {
    if (!awaitingCheck) return;
    const id = setInterval(() => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.tournamentGamesStatus(status.tournament_id) });
    }, RECHECK_MS);
    return () => clearInterval(id);
  }, [awaitingCheck, queryClient, status.tournament_id]);

  if (!enabled) return null;

  let text: string;
  if (waitText) {
    text = waitText;
  } else if (msLeft === null) {
    text = 'авто-раунд включён';
  } else if (msLeft <= 0) {
    text = 'раунд стартует';
  } else {
    const totalSec = Math.floor(msLeft / 1000);
    const h = Math.floor(totalSec / 3600);
    const m = Math.floor((totalSec % 3600) / 60);
    const sec = totalSec % 60;
    const mm = String(m).padStart(2, '0');
    const ss = String(sec).padStart(2, '0');
    text = `следующий раунд через ${h > 0 ? `${h}:` : ''}${mm}:${ss}`;
  }

  return (
    <span
      className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-medium text-primary-300"
      style={{ backgroundColor: 'rgba(139,92,246,0.12)', border: '1px solid rgba(139,92,246,0.35)' }}
      title="Авто-раунды включены: новый раунд запускается автоматически, когда прошёл интервал и появились новые программы"
    >
      <ClockIcon className="w-3.5 h-3.5" />
      {text}
    </span>
  );
}
