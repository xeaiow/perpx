package history

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

func TestUpdate_FetchedSortsDescAndRenders(t *testing.T) {
	m := New(nil)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)
	m, _ = m.Update(uimsg.HistoryFetchedMsg{
		Items: []exchange.ClosedPosition{
			{Exchange: "bybit", Symbol: "BTCUSDT", Side: exchange.SideLong, Size: 0.1, RealizedPnL: 5, CloseTime: older},
			{Exchange: "okx", Symbol: "ETHUSDT", Side: exchange.SideShort, Size: 2, RealizedPnL: -3, CloseTime: newer},
		},
		At: time.Now(),
	})
	if m.items[0].Symbol != "ETHUSDT" {
		t.Errorf("expected ETHUSDT first (newer), got %q", m.items[0].Symbol)
	}
	out := m.View()
	if !strings.Contains(out, "BTCUSDT") || !strings.Contains(out, "ETHUSDT") {
		t.Errorf("view missing symbols")
	}
}

func TestUpdate_EmptyShowsNoHistory(t *testing.T) {
	m := New(nil)
	// 直接走 update 拿到 lastFetch 但 items=nil，模擬「拉成功但無資料」
	m, _ = m.Update(uimsg.HistoryFetchedMsg{At: time.Now()})
	out := m.View()
	if !strings.Contains(out, "No history") {
		t.Errorf("expected empty placeholder, got:\n%s", out)
	}
}

func TestFilter_BinanceMissingEntryExitRendersDash(t *testing.T) {
	m := New(nil)
	m, _ = m.Update(uimsg.HistoryFetchedMsg{
		Items: []exchange.ClosedPosition{
			{Exchange: "binance", Symbol: "BTCUSDT", RealizedPnL: 10, CloseTime: time.Now()},
		},
	})
	out := m.View()
	if !strings.Contains(out, "—") {
		t.Errorf("expected dash for missing fields, got:\n%s", out)
	}
}

func TestCycleFilter_AdvancesThroughExchanges(t *testing.T) {
	m := New(map[string]exchange.Exchange{
		"bybit": fakeEx{}, "okx": fakeEx{},
	})
	if m.filter != "" {
		t.Fatalf("initial filter = %q", m.filter)
	}
	m.cycleFilter()
	if m.filter == "" {
		t.Errorf("expected filter advance, still %q", m.filter)
	}
}

type fakeEx struct{}

func (fakeEx) Name() string { return "fake" }
func (fakeEx) Positions(context.Context) ([]exchange.Position, error) {
	return nil, nil
}
func (fakeEx) AvailableBalance(context.Context) (float64, error) { return 0, nil }
func (fakeEx) History(context.Context, time.Time) ([]exchange.ClosedPosition, error) {
	return nil, nil
}
func (fakeEx) ClosePosition(context.Context, exchange.CloseRequest) (exchange.CloseResult, error) {
	return exchange.CloseResult{}, nil
}
