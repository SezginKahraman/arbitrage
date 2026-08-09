package exchanges

import "testing"

func TestKrakenCOTISymbolConversion(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"COTI outbound", convertToKrakenSymbol("COTIUSDT"), "PF_COTIUSD"},
		{"COTI inbound", convertFromKrakenSymbol("PF_COTIUSD"), "COTIUSDT"},
		{"BTC outbound", convertToKrakenSymbol("BTCUSDT"), "PF_XBTUSD"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Fatalf("got %q, want %q", test.got, test.want)
			}
		})
	}
}
