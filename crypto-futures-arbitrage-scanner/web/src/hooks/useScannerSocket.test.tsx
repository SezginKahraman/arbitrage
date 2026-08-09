import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { SocketLike } from './useScannerSocket';
import { useScannerSocket } from './useScannerSocket';

class FakeSocket implements SocketLike {
  onopen: (() => void) | null = null;
  onmessage: ((event: MessageEvent<string>) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();

  open() {
    this.onopen?.();
  }

  message(value: unknown) {
    this.onmessage?.({ data: JSON.stringify(value) } as MessageEvent<string>);
  }

  disconnect() {
    this.onclose?.();
  }
}

describe('useScannerSocket', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('connects, reduces messages, and closes the socket on unmount', () => {
    const socket = new FakeSocket();
    const { result, unmount } = renderHook(() =>
      useScannerSocket({ socketFactory: () => socket, now: () => 5_000 }),
    );

    act(() => socket.open());
    expect(result.current.connection).toBe('live');

    act(() => socket.message({ type: 'prices', prices: { COTIUSDT: { binance_spot: 0.01140723 } } }));
    expect(result.current.prices.COTIUSDT.binance_spot.price).toBe(0.01140723);

    unmount();
    expect(socket.close).toHaveBeenCalledOnce();
  });

  it('reconnects with a bounded delay after an unexpected close', () => {
    vi.useFakeTimers();
    const sockets: FakeSocket[] = [];
    const { unmount } = renderHook(() =>
      useScannerSocket({
        socketFactory: () => {
          const socket = new FakeSocket();
          sockets.push(socket);
          return socket;
        },
        reconnectBaseMs: 100,
      }),
    );

    act(() => sockets[0].disconnect());
    expect(sockets).toHaveLength(1);

    act(() => vi.advanceTimersByTime(100));
    expect(sockets).toHaveLength(2);

    act(() => sockets[1].disconnect());
    act(() => vi.advanceTimersByTime(199));
    expect(sockets).toHaveLength(2);
    act(() => vi.advanceTimersByTime(1));
    expect(sockets).toHaveLength(3);

    act(() => sockets[2].open());
    act(() => sockets[2].disconnect());
    act(() => vi.advanceTimersByTime(100));
    expect(sockets).toHaveLength(4);

    act(() => sockets[3].disconnect());
    unmount();
    act(() => vi.advanceTimersByTime(30_000));
    expect(sockets).toHaveLength(4);
  });
});
