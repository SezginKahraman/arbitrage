package exchanges

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

func waitForReconnect(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func closeWebSocketOnCancel(ctx context.Context, conn *websocket.Conn) func() bool {
	return context.AfterFunc(ctx, func() { _ = conn.Close() })
}
