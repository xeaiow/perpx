package positions

import (
	"strings"
	"testing"
	"time"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

func TestUpdate_FetchedMsgPopulatesModel(t *testing.T) {
	m := New(nil)
	m, _ = m.Update(uimsg.PositionsFetchedMsg{
		Positions: []exchange.Position{
			{Exchange: "bybit", Symbol: "BTCUSDT", Side: exchange.SideLong, Size: 0.1, UnrealizedPnL: 5, MarkPrice: 60000},
			{Exchange: "okx", Symbol: "ETHUSDT", Side: exchange.SideShort, Size: 1, UnrealizedPnL: -3, MarkPrice: 3000},
		},
		At: time.Now(),
	})
	if len(m.positions) != 2 {
		t.Fatalf("expected 2, got %d", len(m.positions))
	}
	// sorted by exchange asc: bybit then okx
	if m.positions[0].Exchange != "bybit" {
		t.Errorf("first = %q", m.positions[0].Exchange)
	}
	out := m.View()
	if !strings.Contains(out, "BTCUSDT") || !strings.Contains(out, "ETHUSDT") {
		t.Errorf("view missing symbols:\n%s", out)
	}
	if !strings.Contains(out, "Total uPnL") {
		t.Errorf("view missing status line:\n%s", out)
	}
}

func TestUpdate_EmptyPositionsRenderable(t *testing.T) {
	m := New(nil)
	out := m.View()
	if out == "" {
		t.Errorf("empty view")
	}
}
