// @vitest-environment happy-dom
import { act, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it } from 'vitest';
import { Modal } from './Modal';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function Harness() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button id="opener" onClick={() => setOpen(true)}>открыть</button>
      <Modal open={open} onClose={() => setOpen(false)} title="Диалог">
        <button id="ok" onClick={() => setOpen(false)}>ок</button>
      </Modal>
    </>
  );
}

it('держит фокус внутри диалога и возвращает его на открывший элемент', async () => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<Harness />));

  const opener = document.getElementById('opener')!;
  opener.focus();
  await act(async () => opener.click());

  const dialog = document.querySelector<HTMLElement>('[role="dialog"]')!;
  expect(dialog.getAttribute('aria-labelledby')).toBe(dialog.querySelector('h2')!.id);
  expect(document.activeElement).toBe(dialog);

  // Tab с последней кнопки переходит на первую (крестик в заголовке).
  const ok = document.getElementById('ok')!;
  ok.focus();
  await act(async () => {
    ok.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }));
  });
  expect(document.activeElement).toBe(dialog.querySelector('[aria-label="Закрыть"]'));

  await act(async () => ok.click());
  expect(document.querySelector('[role="dialog"]')).toBeNull();
  expect(document.activeElement).toBe(opener);

  root.unmount();
});
