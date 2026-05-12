package accounts

import (
	"strings"
	"testing"
	"time"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

func TestUpdate_RendersEquityAndTotal(t *testing.T) {
	m := New(map[string]exchange.Exchange{
		"bybit": nil, "okx": nil,
	})
	m, _ = m.Update(uimsg.AccountsFetchedMsg{
		Equity: map[string]float64{
			"bybit": 1000,
			"okx":   500,
		},
		At: time.Now(),
	})
	out := m.View()
	if !strings.Contains(out, "1000.00") {
		t.Errorf("missing bybit equity:\n%s", out)
	}
	if !strings.Contains(out, "500.00") {
		t.Errorf("missing okx equity:\n%s", out)
	}
	if !strings.Contains(out, "1500.00") {
		t.Errorf("missing total:\n%s", out)
	}
}

func TestUpdate_FailureShowsDash(t *testing.T) {
	m := New(map[string]exchange.Exchange{"bybit": nil})
	// 模擬錯誤、且沒有 equity 資料
	m, _ = m.Update(uimsg.AccountsFetchedMsg{
		Errors: map[string]error{"bybit": errSentinel("oops")},
		At:     time.Now(),
	})
	out := m.View()
	if !strings.Contains(out, "—") {
		t.Errorf("expected dash for failed exchange, got:\n%s", out)
	}
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
