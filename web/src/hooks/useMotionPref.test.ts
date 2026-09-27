// @vitest-environment happy-dom
import { describe, it, expect, beforeEach, vi } from 'vitest';

// системная настройка с возможностью переключить её в тесте
function stubSystem(matches: boolean) {
  const handlers: Array<() => void> = [];
  const mql = {
    matches,
    addEventListener: (_: string, h: () => void) => handlers.push(h),
  };
  vi.stubGlobal('matchMedia', () => mql);
  return (next: boolean) => {
    mql.matches = next;
    handlers.forEach((h) => h());
  };
}

async function load() {
  vi.resetModules();
  return import('./useMotionPref');
}

describe('useMotionPref', () => {
  beforeEach(() => {
    localStorage.clear();
    delete document.documentElement.dataset.motion;
  });

  it('следует системе, пока нет выбора, и пишет data-motion', async () => {
    const setSystem = stubSystem(true);
    const m = await load();
    expect(m.isMotionReduced()).toBe(true);
    expect(document.documentElement.dataset.motion).toBe('reduced');

    setSystem(false);
    expect(m.isMotionReduced()).toBe(false);
    expect(document.documentElement.dataset.motion).toBeUndefined();
  });

  it('переключатель перекрывает систему, совпавший с ней выбор снимается', async () => {
    const setSystem = stubSystem(true);
    const m = await load();

    m.setMotionReduced(false);
    expect(m.isMotionReduced()).toBe(false);
    expect(localStorage.getItem('motion')).toBe('full');
    setSystem(true);
    expect(m.isMotionReduced()).toBe(false);

    // выбор переживает перезагрузку
    expect((await load()).isMotionReduced()).toBe(false);

    const again = await load();
    again.setMotionReduced(true);
    expect(localStorage.getItem('motion')).toBeNull();
    expect(document.documentElement.dataset.motion).toBe('reduced');
  });
});
