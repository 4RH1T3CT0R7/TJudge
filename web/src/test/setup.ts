import { vi } from 'vitest';

// Node 25+ держит свой глобальный localStorage (без --localstorage-file он
// undefined), и тот закрывает хранилище happy-dom.
if (typeof window !== 'undefined' && typeof localStorage === 'undefined') {
  vi.stubGlobal('localStorage', new Storage());
}
