import { useConfirmStore } from '../../store/confirmStore';
import { ExclamationTriangleIcon } from '../icons';
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
    >
      {pending && (
        <>
          <div className="flex gap-3 items-start">
            {pending.danger && (
              <ExclamationTriangleIcon className="w-6 h-6 text-red-400 shrink-0 mt-0.5" />
            )}
            <p className="text-gray-300 whitespace-pre-line">{pending.message}</p>
          </div>
          <div className="flex justify-end gap-3 mt-6">
            <button className="btn btn-secondary" onClick={() => settle(false)} autoFocus>
              {pending.cancelLabel ?? 'Отмена'}
            </button>
            <button
              className={`btn ${pending.danger ? 'btn-danger' : 'btn-primary'}`}
              onClick={() => settle(true)}
            >
              {pending.confirmLabel ?? 'Подтвердить'}
            </button>
          </div>
        </>
      )}
    </Modal>
  );
}
