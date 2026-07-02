import type { ReactNode } from 'react';
import { useEscapeKey } from '../../hooks/useEscapeKey';
import { XMarkIcon } from '../icons';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children: ReactNode;
  /** Tailwind-класс максимальной ширины контента. */
  maxWidth?: string;
}

// Общая модалка на классах .modal-backdrop/.modal-content (index.css):
// клик по фону и Escape закрывают, клик по контенту — нет.
export function Modal({ open, onClose, title, children, maxWidth = 'max-w-md' }: ModalProps) {
  useEscapeKey(onClose, open);

  if (!open) return null;

  return (
    <div className="modal-backdrop" onClick={onClose} role="dialog" aria-modal="true">
      <div
        className={`modal-content w-full ${maxWidth} p-6 m-4`}
        onClick={(e) => e.stopPropagation()}
      >
        {title !== undefined && (
          <div className="flex items-center justify-between mb-6">
            <h2 className="text-xl font-bold text-gray-100">{title}</h2>
            <button
              onClick={onClose}
              aria-label="Закрыть"
              className="p-2 hover:bg-gray-800 rounded-lg transition-colors"
            >
              <XMarkIcon />
            </button>
          </div>
        )}
        {children}
      </div>
    </div>
  );
}
