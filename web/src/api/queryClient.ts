import { QueryClient } from '@tanstack/react-query';

// Единый QueryClient приложения. Отдельный модуль, чтобы authStore мог
// очистить кэш при выходе, не импортируя App.
// retry: false - ApiClient уже делает exponential retry для GET/5xx/429
// в axios-интерсепторе; дублировать ретраи на уровне Query не нужно.
// staleTime 15s - турнирные данные обновляются WS-инвалидациями
// (useTournamentLive), фоновое поведение по умолчанию консервативное.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      staleTime: 15_000,
      refetchOnWindowFocus: true,
    },
  },
});
