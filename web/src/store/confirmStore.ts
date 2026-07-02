import { create } from 'zustand';

export interface ConfirmOptions {
  title?: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Красная кнопка подтверждения для необратимых действий. */
  danger?: boolean;
}

interface PendingConfirm extends ConfirmOptions {
  resolve: (confirmed: boolean) => void;
}

interface ConfirmStore {
  pending: PendingConfirm | null;
  ask: (options: ConfirmOptions) => Promise<boolean>;
  settle: (confirmed: boolean) => void;
}

// Промис-замена window.confirm: `if (!(await confirmDialog({...}))) return;`
// Рендерится одним <ConfirmDialogHost /> в App (как ToastContainer).
export const useConfirmStore = create<ConfirmStore>()((set, get) => ({
  pending: null,

  ask: (options) =>
    new Promise<boolean>((resolve) => {
      // Параллельный второй вызов отменяет первый: на экране одна модалка.
      get().pending?.resolve(false);
      set({ pending: { ...options, resolve } });
    }),

  settle: (confirmed) => {
    get().pending?.resolve(confirmed);
    set({ pending: null });
  },
}));

export function confirmDialog(options: ConfirmOptions): Promise<boolean> {
  return useConfirmStore.getState().ask(options);
}
