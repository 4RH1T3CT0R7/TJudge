// @vitest-environment happy-dom
import { act, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { expect, it } from 'vitest';
import { Modal } from './Modal';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function Harness({ closeOnBackdrop }: { closeOnBackdrop?: boolean }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button id="opener" onClick={() => setOpen(true)}>открыть</button>
      <Modal open={open} onClose={() => setOpen(false)} title="Диалог" closeOnBackdrop={closeOnBackdrop}>
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

it('закрывается по фону, только если нажатие началось на фоне', async () => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<Harness />));
  await act(async () => document.getElementById('opener')!.click());

  const backdrop = document.querySelector<HTMLElement>('.modal-backdrop')!;
  const ok = document.getElementById('ok')!;
  // выделение начато внутри диалога и отпущено над фоном
  await act(async () => {
    ok.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    backdrop.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
  expect(document.querySelector('[role="dialog"]')).not.toBeNull();

  await act(async () => {
    backdrop.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    backdrop.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
  expect(document.querySelector('[role="dialog"]')).toBeNull();

  root.unmount();
});

it('с closeOnBackdrop={false} клик по фону не закрывает', async () => {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<Harness closeOnBackdrop={false} />));
  await act(async () => document.getElementById('opener')!.click());

  const backdrop = document.querySelector<HTMLElement>('.modal-backdrop')!;
  await act(async () => {
    backdrop.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    backdrop.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
  expect(document.querySelector('[role="dialog"]')).not.toBeNull();

  root.unmount();
});

it('Escape закрывает только верхний из открытых диалогов', async () => {
  function Stacked() {
    const [outer, setOuter] = useState(true);
    const [inner, setInner] = useState(true);
    return (
      <>
        <Modal open={outer} onClose={() => setOuter(false)} title="Форма">
          <p>форма</p>
        </Modal>
        <Modal open={inner} onClose={() => setInner(false)} title="Подтверждение">
          <p>подтверждение</p>
        </Modal>
      </>
    );
  }
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<Stacked />));
  expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(2);

  const escape = () => document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
  await act(async () => escape());
  const left = document.querySelectorAll('[role="dialog"]');
  expect(left).toHaveLength(1);
  expect(left[0].textContent).toContain('форма');

  await act(async () => escape());
  expect(document.querySelector('[role="dialog"]')).toBeNull();
  root.unmount();
});
