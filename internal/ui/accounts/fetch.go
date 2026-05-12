package accounts

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

// FetchCmd 同時抓 AvailableBalance + Positions，計算 equity = available + Σ notional。
func FetchCmd(exs map[string]exchange.Exchange) tea.Cmd {
	return func() tea.Msg {
		var (
			mu     sync.Mutex
			equity = make(map[string]float64)
			errs   = make(map[string]error)
			wg     sync.WaitGroup
		)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for name, ex := range exs {
			wg.Add(1)
			go func(name string, ex exchange.Exchange) {
				defer wg.Done()
				avail, err := ex.AvailableBalance(ctx)
				if err != nil {
					mu.Lock()
					errs[name] = err
					mu.Unlock()
					return
				}
				ps, err := ex.Positions(ctx)
				if err != nil {
					// 仍然顯示 available，但記錯
					mu.Lock()
					equity[name] = avail
					errs[name] = err
					mu.Unlock()
					return
				}
				var notional float64
				for _, p := range ps {
					notional += p.Notional
				}
				mu.Lock()
				equity[name] = avail + notional
				mu.Unlock()
			}(name, ex)
		}
		wg.Wait()
		return uimsg.AccountsFetchedMsg{Equity: equity, Errors: errs, At: time.Now()}
	}
}
