import { useEffect, useState } from 'react';
import { teamKey, type StandingRow } from '../utils/liveStandings';

const SHOW_MS = 8000;

/**
 * Сдвиги мест за последнее изменение таблицы: ключ команды → на сколько мест
 * поднялась (>0) или опустилась (<0). Держатся 8 с, следующее изменение
 * заменяет их. Первый показ и команды без места сдвигов не дают.
 */
export function usePlaceChanges(rows: StandingRow[]): Map<string, number> {
  // подпись мест сравнивается строкой: массив строк пересобирается на каждый рендер
  const sig = rows.map((r) => `${teamKey(r.entry)}:${r.place ?? ''}`).join('|');
  const [prevSig, setPrevSig] = useState(sig);
  const [changes, setChanges] = useState<Map<string, number>>(() => new Map());

  if (sig !== prevSig) {
    setPrevSig(sig);
    const was = new Map(prevSig.split('|').map((s) => s.split(':') as [string, string]));
    const diff = new Map<string, number>();
    for (const r of rows) {
      const before = Number(was.get(teamKey(r.entry)) || NaN);
      if (r.place !== null && before && before !== r.place) diff.set(teamKey(r.entry), before - r.place);
    }
    // пустой diff тоже снимает прежние стрелки: они относились к другому порядку
    if (diff.size > 0 || changes.size > 0) setChanges(diff);
  }

  useEffect(() => {
    if (changes.size === 0) return;
    const t = setTimeout(() => setChanges(new Map()), SHOW_MS);
    return () => clearTimeout(t);
  }, [changes]);

  return changes;
}
