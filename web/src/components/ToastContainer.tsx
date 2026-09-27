import { useEffect, useState } from 'react';
import { useToastStore, type Toast } from '../store/toastStore';
import { XMarkIcon } from './icons';

// Тоны сообщений — та же шкала, что у бейджей статусов: красный, зелёный, синий.
const TONE: Record<Toast['type'], { tag: string; text: string; border: string }> = {
  error: { tag: '✕ ERR', text: 'text-red-400', border: 'border-l-red-500' },
  success: { tag: '✓ OK', text: 'text-green-400', border: 'border-l-green-500' },
  info: { tag: '· NOTE', text: 'text-blue-300', border: 'border-l-blue-500' },
};

function ToastItem({ toast }: { toast: Toast }) {
  const removeToast = useToastStore((s) => s.removeToast);
  const [visible, setVisible] = useState(false);
  const tone = TONE[toast.type];

  useEffect(() => {
    // выезд со следующего кадра
    const raf = requestAnimationFrame(() => setVisible(true));
    return () => cancelAnimationFrame(raf);
  }, []);

  const handleDismiss = () => {
    setVisible(false);
    // удаление из стора после анимации ухода
    setTimeout(() => removeToast(toast.id), 200);
  };

  return (
    // ошибка — alert (озвучивается сразу), остальное читается вежливым live-регионом контейнера
    <div
      role={toast.type === 'error' ? 'alert' : undefined}
      className={`
        pointer-events-auto px-3 py-2 rounded
        bg-gray-950 border border-gray-800 border-l-2 ${tone.border}
        shadow-lg shadow-black/40 text-sm
        transition-[translate,opacity] duration-(--dur-base)
        ${visible ? 'translate-x-0 opacity-100' : 'translate-x-full opacity-0'}
      `}
    >
      {/* Скринридер читает только эту копию: сообщение раньше кнопки закрытия, а новый
          key на повторе заново вставляет её в live-регион, и повтор озвучивается целиком */}
      <span key={toast.count} className="sr-only">
        {toast.count > 1 ? `${toast.message} (повтор ${toast.count})` : toast.message}
      </span>
      <div className="flex items-center gap-2 font-mono text-xs">
        <span aria-hidden="true" className={`font-bold ${tone.text}`}>{tone.tag}</span>
        <time aria-hidden="true" className="text-gray-500" dateTime={new Date(toast.at).toISOString()}>
          {new Date(toast.at).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}
        </time>
        {toast.count > 1 && <span aria-hidden="true" className="text-gray-300">×{toast.count}</span>}
        <button
          type="button"
          onClick={handleDismiss}
          aria-label="Закрыть уведомление"
          className="ml-auto -mr-1 p-1 rounded text-gray-500 hover:text-gray-200 transition-colors"
        >
          <XMarkIcon className="w-4 h-4" />
        </button>
      </div>
      <p aria-hidden="true" className="mt-0.5 break-words leading-snug text-gray-100">{toast.message}</p>
    </div>
  );
}

// Тосты как строки лога: метка тона, время, счётчик повторов «×N»; не больше трёх (toastStore).
export function ToastContainer() {
  const toasts = useToastStore((s) => s.toasts);

  // контейнер есть всегда: live-регион должен существовать до появления сообщений.
  // Ниже шапки: ошибка висит до закрытия и не должна закрывать «Выйти»
  return (
    <div
      aria-live="polite"
      className="fixed top-20 right-4 z-[9999] flex w-[min(24rem,calc(100vw-2rem))] flex-col gap-2 pointer-events-none"
    >
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} />
      ))}
    </div>
  );
}
