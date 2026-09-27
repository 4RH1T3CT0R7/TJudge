import { useState } from 'react';
import { useConfirmStore } from '../../store/confirmStore';
import type { ConfirmOptions } from '../../store/confirmStore';
import { ExclamationTriangleIcon } from '../icons';
import { Field } from './Field';
import { Modal } from './Modal';

// Глобальный хост диалога подтверждения (см. confirmStore.ts).
// Использование: const ok = await confirmDialog({ message: '...', danger: true });
export function ConfirmDialogHost() {
  const pending = useConfirmStore((s) => s.pending);
  const settle = useConfirmStore((s) => s.settle);

  return (
    <Modal
      open={pending !== null}
      onClose={() => settle(false)}
      title={pending?.title ?? 'Подтверждение'}
      maxWidth={pending?.details ? 'max-w-lg' : undefined}
    >
      {pending && <ConfirmBody key={pending.id} options={pending} settle={settle} />}
    </Modal>
  );
}

// Последствия действия построчно, перенос строки уходит под текст, а не под маркер.
export function Consequences({ lines, className = '' }: { lines: string[]; className?: string }) {
  return (
    <ul className={`${className} list-['-_'] space-y-1.5 rounded border border-gray-800 bg-gray-950 py-2 pl-8 pr-3 font-mono text-sm text-gray-300 marker:text-gray-500`}>
      {lines.map((line) => (
        <li key={line}>{line}</li>
      ))}
    </ul>
  );
}

// Фокус сразу на «Отмена»: Enter по привычке не подтверждает. При вводе
// названия фокус в поле, Enter подтверждает только совпавший текст.
function ConfirmBody({ options, settle }: { options: ConfirmOptions; settle: (confirmed: boolean) => void }) {
  const [typed, setTyped] = useState('');
  const expected = options.typeToConfirm?.trim();
  const locked = expected !== undefined && typed.trim() !== expected;

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (!locked) settle(true);
      }}
    >
      <div className="flex gap-3 items-start">
        {options.danger && (
          <ExclamationTriangleIcon className="w-6 h-6 text-red-400 shrink-0 mt-0.5" />
        )}
        <p className="text-gray-300 whitespace-pre-line">{options.message}</p>
      </div>
      {options.details && options.details.length > 0 && (
        <Consequences lines={options.details} className="mt-4" />
      )}
      {expected !== undefined && (
        <Field
          className="mt-4"
          label={
            <>
              Чтобы подтвердить, введите{' '}
              <span className="block mt-1 font-mono text-gray-100 break-words">{expected}</span>
            </>
          }
        >
          {(control) => (
            <input
              {...control}
              className="input"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              spellCheck={false}
              autoFocus
            />
          )}
        </Field>
      )}
      <div className="flex justify-end gap-3 mt-6">
        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => settle(false)}
          autoFocus={expected === undefined}
        >
          {options.cancelLabel ?? 'Отмена'}
        </button>
        <button
          type="submit"
          disabled={locked}
          className={`btn ${options.danger ? 'btn-danger-fill' : 'btn-primary'}`}
        >
          {options.confirmLabel ?? 'Подтвердить'}
        </button>
      </div>
    </form>
  );
}
