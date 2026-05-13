package positions

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/numfmt"
	"github.com/yourname/poscli/internal/ui/styles"
)

// View 渲染 positions tab：bubbles table + 狀態列。
//
// modal 由 root app 在這個 view 上面疊加；本函式不負責 modal。
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.tbl.View())
	b.WriteString("\n")

	if m.loading && len(m.positions) == 0 {
		b.WriteString(styles.Dim.Render("Loading..."))
		b.WriteString("\n")
	}

	var totalPnL float64
	for _, p := range m.positions {
		totalPnL += p.UnrealizedPnL
	}

	// Errors area
	if len(m.errors) > 0 {
		for name, err := range m.errors {
			b.WriteString(styles.ErrorText.Render(fmt.Sprintf("✗ %s: %v", name, err)))
			b.WriteString("\n")
		}
	}

	stats := fmt.Sprintf("Total uPnL: %s USDT     %d positions across %d exchanges",
		numfmt.F(totalPnL), len(m.positions), len(m.exs))
	b.WriteString(styles.Pnl(totalPnL).Render(stats))
	b.WriteString("\n")
	if !m.lastFetch.IsZero() {
		ago := time.Since(m.lastFetch).Round(time.Second)
		b.WriteString(styles.Dim.Render(fmt.Sprintf("Updated %s ago", ago)))
		b.WriteString("\n")
	}
	b.WriteString(styles.Dim.Render("[↑↓/jk] navigate  [x] close selected  [tab] switch view  [r] refresh  [q] quit"))
	return b.String()
}

// unmatchedSymbols 對每個 normalized symbol，回傳 true 代表「不是健康 hedge」。
//
// 健康 hedge 的嚴格定義：對該 symbol 恰好有 2 筆持倉、且恰為 1 long + 1 short。
// 其餘所有情況一律視為 unmatched，包括：
//   - 單腿（1 筆）
//   - 同方向多腿（2L、3L、2S 等）
//   - long/short 都有但條數不為 2（如 2L + 1S）
func unmatchedSymbols(ps []exchange.Position) map[string]bool {
	type counts struct{ long, short int }
	bySymbol := map[string]counts{}
	for _, p := range ps {
		c := bySymbol[p.Symbol]
		switch p.Side {
		case exchange.SideLong:
			c.long++
		case exchange.SideShort:
			c.short++
		}
		bySymbol[p.Symbol] = c
	}
	out := make(map[string]bool, len(bySymbol))
	for sym, c := range bySymbol {
		out[sym] = !(c.long == 1 && c.short == 1)
	}
	return out
}
