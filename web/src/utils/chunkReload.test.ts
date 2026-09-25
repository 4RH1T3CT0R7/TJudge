import { afterEach, describe, expect, it, vi } from 'vitest';
import { shouldReloadOnChunkError } from './chunkReload';

describe('shouldReloadOnChunkError', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('перезагружает не чаще раза в 10 с', () => {
    const store = new Map<string, string>();
    vi.stubGlobal('sessionStorage', {
      getItem: (key: string) => store.get(key) ?? null,
      setItem: (key: string, value: string) => store.set(key, value),
    });

    expect(shouldReloadOnChunkError(100_000)).toBe(true);
    expect(shouldReloadOnChunkError(105_000)).toBe(false);
    expect(shouldReloadOnChunkError(110_000)).toBe(true);
  });

  it('без доступа к sessionStorage не перезагружает', () => {
    vi.stubGlobal('sessionStorage', {
      getItem: () => {
        throw new Error('SecurityError');
      },
    });

    expect(shouldReloadOnChunkError(100_000)).toBe(false);
  });
});
