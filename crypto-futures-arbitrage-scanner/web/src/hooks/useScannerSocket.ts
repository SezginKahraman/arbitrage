import { useEffect, useRef, useState } from 'react';

import type { ScannerState } from '../app/types';
import { createInitialScannerState, reduceScannerMessage } from '../lib/market-state';

export interface SocketLike {
  onopen: (() => void) | null;
  onmessage: ((event: MessageEvent<string>) => void) | null;
  onclose: (() => void) | null;
  onerror: (() => void) | null;
  close(): void;
}

interface ScannerSocketOptions {
  socketFactory?: (url: string) => SocketLike;
  now?: () => number;
  reconnectBaseMs?: number;
}

function scannerSocketUrl(): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${protocol}//${window.location.host}/ws`;
}

export function useScannerSocket(options: ScannerSocketOptions = {}): ScannerState {
  const [state, setState] = useState<ScannerState>(createInitialScannerState);
  const optionsRef = useRef(options);
  optionsRef.current = options;

  useEffect(() => {
    let socket: SocketLike | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let reconnectAttempt = 0;
    let stopped = false;

    const connect = () => {
      if (stopped) return;

      const socketFactory =
        optionsRef.current.socketFactory ?? ((url: string) => new WebSocket(url) as unknown as SocketLike);
      const now = optionsRef.current.now ?? Date.now;
      const nextSocket = socketFactory(scannerSocketUrl());
      socket = nextSocket;

      nextSocket.onopen = () => {
        reconnectAttempt = 0;
        setState((current) => ({ ...current, connection: 'live' }));
      };

      nextSocket.onmessage = (event) => {
        try {
          const message: unknown = JSON.parse(event.data);
          setState((current) => reduceScannerMessage(current, message, now()));
        } catch {
          // Ignore malformed frames and preserve the last valid market state.
        }
      };

      nextSocket.onerror = () => {
        setState((current) => ({ ...current, connection: 'offline' }));
      };

      nextSocket.onclose = () => {
        if (stopped) return;
        setState((current) => ({ ...current, connection: 'reconnecting', opportunities: [] }));
        const baseDelay = optionsRef.current.reconnectBaseMs ?? 1_000;
        const delay = Math.min(baseDelay * 2 ** reconnectAttempt, 30_000);
        reconnectAttempt += 1;
        reconnectTimer = setTimeout(connect, delay);
      };
    };

    connect();

    return () => {
      stopped = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      socket?.close();
    };
  }, []);

  return state;
}
