package positions

import (
	"testing"

	"github.com/yourname/poscli/internal/exchange"
)

func TestUnmatchedSymbols(t *testing.T) {
	cases := []struct {
		name string
		in   []exchange.Position
		want map[string]bool
	}{
		{
			name: "single leg long",
			in: []exchange.Position{
				{Symbol: "AIUSDT", Side: exchange.SideLong},
			},
			want: map[string]bool{"AIUSDT": true},
		},
		{
			name: "balanced hedge: 1 long + 1 short across exchanges",
			in: []exchange.Position{
				{Symbol: "AIUSDT", Side: exchange.SideLong, Exchange: "binance"},
				{Symbol: "AIUSDT", Side: exchange.SideShort, Exchange: "okx"},
			},
			want: map[string]bool{"AIUSDT": false},
		},
		{
			name: "two same-side legs are still unmatched",
			in: []exchange.Position{
				{Symbol: "AIUSDT", Side: exchange.SideLong, Exchange: "binance"},
				{Symbol: "AIUSDT", Side: exchange.SideLong, Exchange: "okx"},
			},
			want: map[string]bool{"AIUSDT": true},
		},
		{
			name: "three legs 2L+1S: unmatched (not exactly 1L+1S)",
			in: []exchange.Position{
				{Symbol: "AIUSDT", Side: exchange.SideLong, Exchange: "binance"},
				{Symbol: "AIUSDT", Side: exchange.SideLong, Exchange: "bitget"},
				{Symbol: "AIUSDT", Side: exchange.SideShort, Exchange: "okx"},
			},
			want: map[string]bool{"AIUSDT": true},
		},
		{
			name: "multiple symbols mixed",
			in: []exchange.Position{
				{Symbol: "BTCUSDT", Side: exchange.SideLong, Exchange: "binance"},
				{Symbol: "BTCUSDT", Side: exchange.SideShort, Exchange: "okx"},
				{Symbol: "ETHUSDT", Side: exchange.SideLong, Exchange: "binance"},
			},
			want: map[string]bool{"BTCUSDT": false, "ETHUSDT": true},
		},
	}
	for _, c := range cases {
		got := unmatchedSymbols(c.in)
		if len(got) != len(c.want) {
			t.Errorf("%s: len = %d, want %d (got %v)", c.name, len(got), len(c.want), got)
			continue
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("%s: %s = %v, want %v", c.name, k, got[k], v)
			}
		}
	}
}
