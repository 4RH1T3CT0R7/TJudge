import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useToastStore } from './toastStore';

const add = useToastStore.getState().addToast;
const toasts = () => useToastStore.getState().toasts;

beforeEach(() => {
  vi.useFakeTimers();
  useToastStore.setState({ toasts: [] });
});
afterEach(() => vi.useRealTimers());

it('схлопывает повторы в одну строку со счётчиком и перезапускает таймер', () => {
  add('сохранено', 'success');
  vi.advanceTimersByTime(4000);
  add('сохранено', 'success');
  expect(toasts()).toHaveLength(1);
  expect(toasts()[0].count).toBe(2);

  // первый таймер на 5 с сброшен повтором
  vi.advanceTimersByTime(4000);
  expect(toasts()).toHaveLength(1);
  vi.advanceTimersByTime(1000);
  expect(toasts()).toHaveLength(0);
});

it('держит не больше трёх, а ошибку не прячет сама', () => {
  add('сеть недоступна', 'error');
  for (const m of ['a', 'b']) add(m);
  add('сеть недоступна', 'error');
  add('c');
  expect(toasts().map((t) => `${t.message}×${t.count}`)).toEqual(['b×1', 'сеть недоступна×2', 'c×1']);

  vi.advanceTimersByTime(60_000);
  expect(toasts().map((t) => t.message)).toEqual(['сеть недоступна']);
});
