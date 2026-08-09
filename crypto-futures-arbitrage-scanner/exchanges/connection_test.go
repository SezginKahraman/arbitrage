package exchanges

import "testing"

func TestPublishConnectionStatusIsTimestampedAndNonBlocking(t *testing.T) {
	statusChan := make(chan ConnectionStatus, 1)
	publishConnectionStatus(statusChan, "binance_spot", true, []string{"BTCUSDT", "COTIUSDT"})
	status := <-statusChan
	if status.Source != "binance_spot" || !status.Connected || status.Timestamp <= 0 || len(status.Symbols) != 2 {
		t.Fatalf("status = %+v", status)
	}

	statusChan <- status
	done := make(chan struct{})
	go func() {
		publishConnectionStatus(statusChan, "binance_spot", false, status.Symbols)
		close(done)
	}()
	<-done
}
