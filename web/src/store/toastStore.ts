import { create } from 'zustand';

export interface Toast {
  id: string;
  message: string;
  type: 'error' | 'success' | 'info';
  duration: number;
  /** Сколько раз пришло это сообщение, на экране «×N». */
  count: number;
  /** Время последнего повтора. */
  at: number;
}

interface ToastStore {
  toasts: Toast[];
  addToast: (message: string, type?: Toast['type'], duration?: number) => void;
  removeToast: (id: string) => void;
}

// На экране не больше трёх: самое старое уходит первым.
const MAX_TOASTS = 3;

let toastCounter = 0;
const timers = new Map<string, ReturnType<typeof setTimeout>>();

function clearTimer(id: string) {
  clearTimeout(timers.get(id));
  timers.delete(id);
}

export const useToastStore = create<ToastStore>()((set, get) => ({
  toasts: [],

  // Ошибка по умолчанию не исчезает сама: её закрывают вручную.
  // Повтор того же сообщения не добавляет строку, а увеличивает счётчик и перезапускает таймер.
  addToast: (message, type = 'info', duration = type === 'error' ? 0 : 5000) => {
    const same = get().toasts.find((t) => t.message === message && t.type === type);
    const id = same?.id ?? `toast-${Date.now()}-${++toastCounter}`;
    const toast: Toast = { id, message, type, duration, count: (same?.count ?? 0) + 1, at: Date.now() };

    const next = [...get().toasts.filter((t) => t.id !== id), toast];
    next.slice(0, -MAX_TOASTS).forEach((t) => clearTimer(t.id));
    set({ toasts: next.slice(-MAX_TOASTS) });

    clearTimer(id);
    if (duration > 0) {
      timers.set(id, setTimeout(() => get().removeToast(id), duration));
    }
  },

  removeToast: (id: string) => {
    clearTimer(id);
    set({ toasts: get().toasts.filter((t) => t.id !== id) });
  },
}));
