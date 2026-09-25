const RELOAD_KEY = 'chunk_reload_at';
const MIN_INTERVAL_MS = 10_000;

// После релиза чанков прошлой сборки на сервере нет, и lazy-импорт в открытой
// вкладке падает. Перезагрузка подтягивает свежий index.html. Повтор не чаще
// раза в 10 с: если сборка сломана по-настоящему, цикла перезагрузок не будет.
export function shouldReloadOnChunkError(now = Date.now()): boolean {
  try {
    const last = Number(sessionStorage.getItem(RELOAD_KEY));
    if (now - last < MIN_INTERVAL_MS) return false;
    sessionStorage.setItem(RELOAD_KEY, String(now));
    return true;
  } catch {
    return false;
  }
}
