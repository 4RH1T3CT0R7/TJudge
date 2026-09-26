import { useEffect, useRef, useState, useCallback } from 'react';
import type { WSMessage } from '../types';
import api, { isTokenExpired } from '../api/client';

// Потолок backoff переподключения. Лимита попыток нет: после деплоя или
// потери сети соединение должно вернуться само, без F5.
const MAX_RECONNECT_DELAY_MS = 30000;

/** Задержка перед попыткой attempt (с 1): 1s, 2s, 4s, ... до 30s. */
export function reconnectDelay(attempt: number): number {
  return Math.min(1000 * 2 ** (attempt - 1), MAX_RECONNECT_DELAY_MS);
}

interface UseWebSocketOptions {
  tournamentId: string;
  onMessage?: (message: WSMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (error: Event) => void;
  enabled?: boolean;
}

export function useWebSocket({
  tournamentId,
  onMessage,
  onOpen,
  onClose,
  onError,
  enabled = false, // По умолчанию выключено, пока сервер не настроен
}: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  // Online/offline awareness для UI + быстрый reconnect при возврате сети.
  const [isOnline, setIsOnline] = useState(
    typeof navigator !== 'undefined' ? navigator.onLine : true
  );
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const connectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttempts = useRef(0);
  const mountedRef = useRef(true);
  // Номер последнего connect: connect после refresh сверяется с ним
  const connectSeqRef = useRef(0);
  // false, если с последнего открытия сокета токен уже обновлялся
  const refreshAllowedRef = useRef(true);

  // Храним значения в refs, чтобы не пересоздавать функции
  const tournamentIdRef = useRef(tournamentId);
  const onMessageRef = useRef(onMessage);
  const onOpenRef = useRef(onOpen);
  const onCloseRef = useRef(onClose);
  const onErrorRef = useRef(onError);

  // Ref для connect-функции, чтобы onclose мог ссылаться на неё без forward declaration
  const connectRef = useRef<(refresh?: boolean) => void>(() => {});

  // Обновляем refs при смене значений
  useEffect(() => {
    tournamentIdRef.current = tournamentId;
    onMessageRef.current = onMessage;
    onOpenRef.current = onOpen;
    onCloseRef.current = onClose;
    onErrorRef.current = onError;
  }, [tournamentId, onMessage, onOpen, onClose, onError]);

  // refresh: true - обновить токен перед подключением, false - не обновлять,
  // не задан - обновить, только если exp уже прошёл
  const connect = useCallback((refresh?: boolean) => {
    // Не коннектимся, если компонент размонтирован
    if (!mountedRef.current) {
      return;
    }

    const currentTournamentId = tournamentIdRef.current;

    // Не коннектимся без tournamentId
    if (!currentTournamentId) {
      return;
    }

    // Всегда берём свежий токен из localStorage
    const token = localStorage.getItem('access_token');
    if (!token) {
      return;
    }

    // Отложенный reconnect больше не нужен: соединение открывается сейчас
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    // Отказ по токену браузер показывает обычным обрывом 1006, поэтому токен
    // обновляется общим с REST single-flight заранее, если exp прошёл, и после
    // сокета, так и не открывшегося (часы клиента отстают, токен отозван).
    // После сетевой ошибки refresh сокет открывается со старым токеном и уходит
    // в backoff; отказ refresh завершает сессию, и токена уже нет.
    const seq = ++connectSeqRef.current;
    if (refresh ?? isTokenExpired(token)) {
      refreshAllowedRef.current = false;
      void api
        .refreshSession()
        .catch(() => {})
        .then(() => {
          if (seq === connectSeqRef.current) connectRef.current(false);
        });
      return;
    }

    // Закрываем существующее соединение, если есть. Его обработчики дальше
    // игнорируются (проверка wsRef.current !== ws), так что close без кода
    // не запланирует лишний reconnect.
    if (wsRef.current) {
      wsRef.current.close(1000);
      wsRef.current = null;
    }

    // Собираем WebSocket URL на основе текущего location
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/api/v1/ws/tournaments/${currentTournamentId}`;

    // Передаём токен через Sec-WebSocket-Protocol header вместо URL query string,
    // чтобы не светить JWT в истории браузера, server logs и referrer headers
    const ws = new WebSocket(wsUrl, [`access_token.${token}`]);
    let opened = false;

    ws.onopen = () => {
      if (!mountedRef.current || wsRef.current !== ws) {
        ws.close(1000);
        return;
      }
      opened = true;
      refreshAllowedRef.current = true;
      setIsConnected(true);
      reconnectAttempts.current = 0;
      onOpenRef.current?.();
    };

    ws.onclose = (event) => {
      // Закрылся старый сокет, уже заменённый новым: состояние принадлежит новому
      if (!mountedRef.current || wsRef.current !== ws) return;

      setIsConnected(false);
      wsRef.current = null;
      onCloseRef.current?.();

      // Чистое закрытие (1000) - по инициативе клиента, переподключение не нужно
      if (event.code !== 1000) {
        // Так и не открывшийся сокет обновляет токен не чаще раза между
        // открытиями: не помог refresh - дело не в токене (сервер лежит,
        // Origin не пропущен), а истёкший exp обновится и так
        const refresh = !opened && refreshAllowedRef.current ? true : undefined;
        reconnectAttempts.current++;
        reconnectTimeoutRef.current = setTimeout(
          () => connectRef.current(refresh),
          reconnectDelay(reconnectAttempts.current)
        );
      }
    };

    ws.onerror = (error) => {
      if (!mountedRef.current || wsRef.current !== ws) return;
      onErrorRef.current?.(error);
    };

    ws.onmessage = (event) => {
      if (!mountedRef.current || wsRef.current !== ws) return;
      try {
        const message = JSON.parse(event.data) as WSMessage;
        onMessageRef.current?.(message);
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    };

    wsRef.current = ws;
  }, []); // Пустые deps - используем refs

  // Держим connectRef в синхроне с connect
  useEffect(() => {
    connectRef.current = connect;
  });

  const disconnect = useCallback(() => {
    connectSeqRef.current++;
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (connectTimeoutRef.current) {
      clearTimeout(connectTimeoutRef.current);
      connectTimeoutRef.current = null;
    }
    if (wsRef.current) {
      wsRef.current.close(1000); // Чистое закрытие
      wsRef.current = null;
    }
    setIsConnected(false);
  }, []);

  const reconnect = useCallback(() => {
    reconnectAttempts.current = 0;
    disconnect();
    connect();
  }, [connect, disconnect]);

  // Слушаем online/offline события браузера.
  // При offline - закрываем WS и выставляем флаг, при online - быстрый reconnect
  // без exponential-backoff (не ждём 16s, пользователь уже вернул сеть).
  useEffect(() => {
    if (typeof window === 'undefined') return;

    const handleOffline = () => {
      setIsOnline(false);
      if (wsRef.current) {
        // WebSocket.close() требует code=1000 или 3000-4999 (WHATWG spec).
        // 1001 ("going away") - это status code от сервера, не валидный аргумент
        // клиентского close(); передача 1001 бросает InvalidAccessError.
        // Используем 1000 (normal closure) как в disconnect() ниже.
        try {
          wsRef.current.close(1000);
        } catch (err) {
          // Defensive: даже если какой-то агент нарушит spec - не ломаем UI.
          console.warn('ws.close on offline failed', err);
        }
      }
    };
    const handleOnline = () => {
      setIsOnline(true);
      if (enabled && tournamentIdRef.current) {
        reconnectAttempts.current = 0;
        // Небольшая задержка, чтобы не биться в ещё не готовый network stack.
        // Через reconnectTimeoutRef: connect() и disconnect() его снимают.
        if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = setTimeout(() => connectRef.current(), 250);
      }
    };

    window.addEventListener('offline', handleOffline);
    window.addEventListener('online', handleOnline);
    return () => {
      window.removeEventListener('offline', handleOffline);
      window.removeEventListener('online', handleOnline);
    };
  }, [enabled]);

  // Коннектимся при изменении tournamentId (с debounce)
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    // Не коннектимся, если отключено
    if (!enabled) {
      return;
    }

    mountedRef.current = true;

    // Отменяем ожидающее подключение
    if (connectTimeoutRef.current) {
      clearTimeout(connectTimeoutRef.current);
    }

    // Рвём существующее соединение
    if (wsRef.current) {
      wsRef.current.close(1000);
      wsRef.current = null;
      setIsConnected(false);
    }

    // Коннектимся только при валидном tournamentId
    if (tournamentId) {
      // Небольшая задержка, чтобы дать React успокоиться и избежать быстрых реконнектов
      connectTimeoutRef.current = setTimeout(() => {
        if (mountedRef.current) {
          connect();
        }
      }, 100);
    }

    return () => {
      mountedRef.current = false;
      disconnect();
    };
  }, [tournamentId, enabled, connect, disconnect]);
  /* eslint-enable react-hooks/set-state-in-effect */

  return { isConnected, isOnline, disconnect, reconnect };
}
