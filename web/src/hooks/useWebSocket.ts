import { useEffect, useRef, useState, useCallback } from 'react';
import type { WSMessage } from '../types';

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
  enabled = false, // Disabled by default until server is properly configured
}: UseWebSocketOptions) {
  const wsRef = useRef<WebSocket | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  // P2.12: online/offline awareness для UI + быстрый reconnect при возврате сети.
  const [isOnline, setIsOnline] = useState(
    typeof navigator !== 'undefined' ? navigator.onLine : true
  );
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const connectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttempts = useRef(0);
  const mountedRef = useRef(true);
  const maxReconnectAttempts = 5;

  // Store values in refs to avoid recreating functions
  const tournamentIdRef = useRef(tournamentId);
  const onMessageRef = useRef(onMessage);
  const onOpenRef = useRef(onOpen);
  const onCloseRef = useRef(onClose);
  const onErrorRef = useRef(onError);

  // Ref to hold the connect function so onclose can reference it without forward declaration
  const connectRef = useRef<() => void>(() => {});

  // Update refs when values change
  useEffect(() => {
    tournamentIdRef.current = tournamentId;
    onMessageRef.current = onMessage;
    onOpenRef.current = onOpen;
    onCloseRef.current = onClose;
    onErrorRef.current = onError;
  }, [tournamentId, onMessage, onOpen, onClose, onError]);

  const connect = useCallback(() => {
    // Don't connect if unmounted
    if (!mountedRef.current) {
      return;
    }

    const currentTournamentId = tournamentIdRef.current;

    // Don't connect if no tournamentId
    if (!currentTournamentId) {
      return;
    }

    // Always get fresh token from localStorage
    const token = localStorage.getItem('access_token');
    if (!token) {
      return;
    }

    // Close existing connection if any
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    // Build WebSocket URL based on current location
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/api/v1/ws/tournaments/${currentTournamentId}`;

    // Send token via Sec-WebSocket-Protocol header instead of URL query string
    // to avoid exposing JWT in browser history, server logs, and referrer headers
    const ws = new WebSocket(wsUrl, [`access_token.${token}`]);

    ws.onopen = () => {
      if (!mountedRef.current) {
        ws.close();
        return;
      }
      setIsConnected(true);
      reconnectAttempts.current = 0;
      onOpenRef.current?.();
    };

    ws.onclose = (event) => {
      if (!mountedRef.current) return;

      setIsConnected(false);
      wsRef.current = null;
      onCloseRef.current?.();

      // Don't reconnect if closed cleanly (code 1000) or max attempts reached
      if (event.code !== 1000 && reconnectAttempts.current < maxReconnectAttempts) {
        reconnectAttempts.current++;
        // Exponential backoff: 1s, 2s, 4s, 8s, 16s
        const delay = Math.min(1000 * Math.pow(2, reconnectAttempts.current - 1), 16000);
        reconnectTimeoutRef.current = setTimeout(() => connectRef.current(), delay);
      }
    };

    ws.onerror = (error) => {
      if (!mountedRef.current) return;
      onErrorRef.current?.(error);
    };

    ws.onmessage = (event) => {
      if (!mountedRef.current) return;
      try {
        const message = JSON.parse(event.data) as WSMessage;
        onMessageRef.current?.(message);
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    };

    wsRef.current = ws;
  }, []); // Empty deps - uses refs

  // Keep connectRef in sync with connect
  useEffect(() => {
    connectRef.current = connect;
  });

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (connectTimeoutRef.current) {
      clearTimeout(connectTimeoutRef.current);
      connectTimeoutRef.current = null;
    }
    if (wsRef.current) {
      wsRef.current.close(1000); // Clean close
      wsRef.current = null;
    }
    setIsConnected(false);
  }, []);

  const reconnect = useCallback(() => {
    reconnectAttempts.current = 0;
    disconnect();
    connect();
  }, [connect, disconnect]);

  // P2.12: слушаем online/offline события браузера.
  // При offline — закрываем WS и выставляем флаг, при online — быстрый reconnect
  // без exponential-backoff (не ждём 16s, пользователь уже вернул сеть).
  useEffect(() => {
    if (typeof window === 'undefined') return;

    const handleOffline = () => {
      setIsOnline(false);
      if (wsRef.current) {
        // UR bug_001: WebSocket.close() требует code=1000 ИЛИ 3000-4999 (WHATWG spec).
        // 1001 ("going away") — это status code от сервера, не валидный аргумент
        // клиентского close(); передача 1001 бросает InvalidAccessError.
        // Используем 1000 (normal closure) как в disconnect() ниже.
        try {
          wsRef.current.close(1000);
        } catch (err) {
          // Defensive: даже если какой-то агент нарушит spec — не ломаем UI.
          console.warn('ws.close on offline failed', err);
        }
      }
    };
    const handleOnline = () => {
      setIsOnline(true);
      if (enabled && tournamentIdRef.current) {
        reconnectAttempts.current = 0;
        // Небольшая задержка, чтобы не биться в ещё не готовый network stack.
        setTimeout(() => connectRef.current(), 250);
      }
    };

    window.addEventListener('offline', handleOffline);
    window.addEventListener('online', handleOnline);
    return () => {
      window.removeEventListener('offline', handleOffline);
      window.removeEventListener('online', handleOnline);
    };
  }, [enabled]);

  // Connect when tournamentId changes (with debounce)
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    // Don't connect if disabled
    if (!enabled) {
      return;
    }

    mountedRef.current = true;

    // Clear any pending connection
    if (connectTimeoutRef.current) {
      clearTimeout(connectTimeoutRef.current);
    }

    // Disconnect existing connection
    if (wsRef.current) {
      wsRef.current.close(1000);
      wsRef.current = null;
      setIsConnected(false);
    }

    // Only connect if we have a valid tournamentId
    if (tournamentId) {
      // Small delay to let React settle and avoid rapid reconnections
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
