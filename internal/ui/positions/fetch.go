package positions

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

// FetchCmd 平行抓所有 adapter.Positions()，結束後回傳一個 PositionsFetchedMsg。
func FetchCmd(exs map[string]exchange.Exchange) tea.Cmd {
	return func() tea.Msg {
		var (
			mu   sync.Mutex
			all  []exchange.Position
			errs = make(map[string]error)
			wg   sync.WaitGroup
		)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for name, ex := range exs {
			wg.Add(1)
			go func(name string, ex exchange.Exchange) {
				defer wg.Done()
				ps, err := ex.Positions(ctx)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs[name] = err
					return
				}
				all = append(all, ps...)
			}(name, ex)
		}
		wg.Wait()
		return uimsg.PositionsFetchedMsg{Positions: all, Errors: errs, At: time.Now()}
	}
}
