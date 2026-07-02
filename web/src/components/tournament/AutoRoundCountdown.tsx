import { useEffect, useState } from 'react';
import { ClockIcon } from '../icons';

interface AutoRoundCountdownProps {
  enabled: boolean;
  intervalSeconds: number;
  lastRunAt: string | null | undefined;
}

// Таймер до следующего авто-раунда: мотивирует успеть загрузить новую
// версию программы. Данные уже есть в публичном /games/status.
export function AutoRoundCountdown({ enabled, intervalSeconds, lastRunAt }: AutoRoundCountdownProps) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!enabled) return;
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [enabled]);

  if (!enabled || intervalSeconds <= 0) return null;

  // Без last_run планировщик стартует от включения — точного времени нет.
  const base = lastRunAt ? new Date(lastRunAt).getTime() : null;
  const nextAt = base !== null ? base + intervalSeconds * 1000 : null;
  const msLeft = nextAt !== null ? nextAt - now : null;

  let text: string;
  if (msLeft === null) {
    text = 'авто-раунд включён';
  } else if (msLeft <= 0) {
    text = 'раунд вот-вот стартует';
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
      className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium text-primary-300"
      style={{ backgroundColor: 'rgba(139,92,246,0.12)', border: '1px solid rgba(139,92,246,0.35)' }}
      title="Авто-раунды включены: новый раунд запускается автоматически"
    >
      <ClockIcon className="w-3.5 h-3.5" />
      {text}
    </span>
  );
}
