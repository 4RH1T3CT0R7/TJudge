import { expect, it } from 'vitest';
import { hasMarkup, ioTone, mdPreview } from './markdown';

it('превью - первый абзац без заголовков и разметки', () => {
  const rules = [
    '# Дилемма заключённого',
    '',
    '| Вы | Он |',
    '|---|---|',
    '',
    'Игроки **одновременно** выбирают `COOPERATE` или *DEFECT*,',
    'см. [правила](/help); поле score_multiplier и _курсив_.',
    '',
    'Второй абзац.',
  ].join('\n');
  expect(mdPreview(rules)).toBe('Игроки одновременно выбирают COOPERATE или DEFECT, см. правила; поле score_multiplier и курсив.');
  expect(mdPreview('')).toBe('');
});

it('строки протокола окрашены по направлению', () => {
  expect(ioTone('← 100   # итерации')).toBe('text-cyan-300');
  expect(ioTone('  → COOPERATE')).toBe('text-primary-300');
  expect(ioTone('...')).toBeUndefined();
});

it('разметка находится, обычный текст с тире и дефисами - нет', () => {
  expect(hasMarkup('Основной турнир. Пять игр, round-robin в обе стороны, итог - сумма очков.\n\nВторой абзац.')).toBe(false);
  for (const md of ['## Правила', '- пункт', '1. пункт', '> цитата', '| a | b |', '**жирный**', 'поле `x`', '[ссылка](/help)', 'см. https://example.org']) {
    expect(hasMarkup(md)).toBe(true);
  }
});
