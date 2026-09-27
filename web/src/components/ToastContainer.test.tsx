// @vitest-environment happy-dom
import { act } from 'react';
import { afterEach, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import { ToastContainer } from './ToastContainer';
import { useToastStore } from '../store/toastStore';

afterEach(() => {
  cleanup();
  useToastStore.setState({ toasts: [] });
});

it('повтор сообщения заново вставляет его текст в live-регион', () => {
  const { container } = render(<ToastContainer />);
  const add = useToastStore.getState().addToast;
  act(() => add('Программа загружена', 'success'));
  const first = container.querySelector('.sr-only')!;
  expect(first.textContent).toBe('Программа загружена');

  act(() => add('Программа загружена', 'success'));
  const second = container.querySelector('.sr-only')!;
  expect(second).not.toBe(first);
  expect(second.textContent).toBe('Программа загружена (повтор 2)');
});
