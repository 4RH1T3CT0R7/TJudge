import { describe, it, expect } from 'vitest';
import { precheckSource } from './precheckFile';

const bytes = (s: string) => new TextEncoder().encode(s);

describe('precheckSource', () => {
  it('C без fflush и чужой язык под расширением .py', () => {
    const c = precheckSource('.c', bytes('#include <stdio.h>\nint main(){ puts("DEFECT"); }'));
    expect(c.error).toBeUndefined();
    expect(c.warnings).toEqual([expect.stringContaining('fflush')]);

    const py = precheckSource('.py', bytes('#include <stdio.h>\nint main(){}'));
    expect(py.warnings[0]).toContain('похоже на C/C++');
  });

  it('бинарник - ошибка, cp1251 и крейт rand - предупреждения', () => {
    expect(precheckSource('.cpp', new Uint8Array([0x7f, 0x45, 0x4c, 0x46, 0, 1])).error).toContain('бинарный');
    // «Привет» в cp1251
    expect(precheckSource('.py', new Uint8Array([0x23, 0x20, 0xcf, 0xf0, 0xe8, 0xe2, 0xe5, 0xf2])).warnings[0]).toContain('UTF-8');
    expect(precheckSource('.rs', bytes('use rand::Rng;\nfn main() {}')).warnings).toEqual([expect.stringContaining('rand')]);
  });

  it('чистый файл без предупреждений', () => {
    const src = 'import sys\nfor line in sys.stdin:\n    print("COOPERATE", flush=True)\n';
    expect(precheckSource('.py', bytes(src))).toEqual({ warnings: [] });
  });
});
