import { expect, it } from 'vitest';
import { extensionOf, precheckSource } from './precheckFile';
import { LANGUAGES } from './starters';

const files = import.meta.glob<string>('../starters/*/*/*', { query: '?raw', import: 'default', eager: true });

it('у каждой игры шаблон на каждом языке, и предпроверка загрузки к нему не придирается', () => {
  const games = new Set(Object.keys(files).map((path) => path.split('/')[2]));
  expect([...games].sort()).toEqual(['dilemma', 'dollar_auction', 'public_goods', 'travelers_dilemma', 'tug_of_war']);
  for (const game of games) {
    for (const lang of LANGUAGES) {
      const name = `${game}/${lang.id}/${lang.file}`;
      const src = files[`../starters/${name}`];
      expect(src, name).toBeDefined();
      expect(precheckSource(extensionOf(lang.file), new TextEncoder().encode(src)).warnings, name).toEqual([]);
    }
  }
  expect(Object.keys(files)).toHaveLength(games.size * LANGUAGES.length);
});

it('строки шаблонов помещаются в блок кода без прокрутки', () => {
  // ~100 знаков в блоке на desktop от 1280 px: длиннее - подписи протокола уходят за край.
  // табуляция в блоке - 4 знака (TerminalOutput)
  for (const [path, src] of Object.entries(files)) {
    for (const line of src.split('\n')) {
      expect(line.replaceAll('\t', '    ').length, `${path}: ${line}`).toBeLessThanOrEqual(88);
    }
  }
});
