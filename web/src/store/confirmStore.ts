import { create } from 'zustand';

export interface ConfirmOptions {
  title?: string;
  message: string;
  /** Последствия построчно, с числами: «удалит 12 матчей». */
  details?: string[];
  /** Кнопка подтверждения откроется, когда введён этот текст (название турнира, команды). */
  typeToConfirm?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Красная кнопка подтверждения для необратимых действий. */
  danger?: boolean;
}

interface PendingConfirm extends ConfirmOptions {
  /** Новый диалог поверх неотвеченного начинается с пустого поля ввода. */
  id: number;
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
      const prev = get().pending;
      prev?.resolve(false);
      set({ pending: { ...options, id: (prev?.id ?? 0) + 1, resolve } });
    }),

  settle: (confirmed) => {
    get().pending?.resolve(confirmed);
    set({ pending: null });
  },
}));

export function confirmDialog(options: ConfirmOptions): Promise<boolean> {
  return useConfirmStore.getState().ask(options);
}
