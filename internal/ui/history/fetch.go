package history

import (
	"context"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

// FetchCmd 平行抓所有 adapter.History()。
func FetchCmd(exs map[string]exchange.Exchange, since time.Time) tea.Cmd {
	return func() tea.Msg {
		var (
			mu   sync.Mutex
			all  []exchange.ClosedPosition
			errs = make(map[string]error)
			wg   sync.WaitGroup
		)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for name, ex := range exs {
			wg.Add(1)
			go func(name string, ex exchange.Exchange) {
				defer wg.Done()
				items, err := ex.History(ctx, since)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs[name] = err
					return
				}
				all = append(all, items...)
			}(name, ex)
		}
		wg.Wait()
		return uimsg.HistoryFetchedMsg{Items: all, Errors: errs, At: time.Now()}
	}
}
