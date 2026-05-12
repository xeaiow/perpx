package positions

import (
	"testing"
	"time"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

func TestSetConfirm_NoSelectionIsNoop(t *testing.T) {
	m := New(nil)
	m.SetConfirm(true)
	if m.ConfirmOpen {
		t.Errorf("expected ConfirmOpen=false when no selection")
	}
}

func TestSetConfirm_WithSelection(t *testing.T) {
	m := New(nil)
	m, _ = m.Update(uimsg.PositionsFetchedMsg{
		Positions: []exchange.Position{{Exchange: "bybit", Symbol: "BTCUSDT", Side: exchange.SideLong, Size: 0.1, MarkPrice: 60000, UnrealizedPnL: 5}},
		At:        time.Now(),
	})
	m.SetConfirm(true)
	if !m.ConfirmOpen {
		t.Fatal("expected ConfirmOpen=true")
	}
	if m.ConfirmTarget == nil || m.ConfirmTarget.Symbol != "BTCUSDT" {
		t.Errorf("ConfirmTarget = %+v", m.ConfirmTarget)
	}
	m.SetConfirm(false)
	if m.ConfirmOpen || m.ConfirmTarget != nil {
		t.Errorf("close: %+v", m)
	}
}
