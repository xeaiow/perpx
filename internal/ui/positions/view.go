package positions

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/numfmt"
	"github.com/yourname/poscli/internal/ui/styles"
)

// View 渲染 positions tab 的表格 + 狀態列。
//
// modal 由 root app 在這個 view 上面疊加；本函式不負責 modal。
func (m Model) View() string {
	var b strings.Builder

	headers := []string{"Exchange", "Symbol", "Side", "Size", "Coin", "Entry", "Mark", "uPnL", "Lev"}
	widths := []int{10, 14, 6, 12, 12, 12, 12, 12, 5}
	b.WriteString(styles.Header.Render(formatRow(headers, widths)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", sumWidth(widths)))
	b.WriteString("\n")

	if m.loading && len(m.positions) == 0 {
		b.WriteString(styles.Dim.Render("Loading..."))
		b.WriteString("\n")
	}

	unmatched := unmatchedSymbols(m.positions)

	var totalPnL float64
	for i, p := range m.positions {
		coinText := "—"
		if p.CoinSize > 0 {
			coinText = trim(numfmt.F(p.CoinSize), 12)
		}
		row := []string{
			p.Exchange,
			p.Symbol,
			string(p.Side),
			trim(numfmt.F(p.Size), 12),
			coinText,
			trim(numfmt.F(p.EntryPrice), 12),
			trim(numfmt.F(p.MarkPrice), 12),
			numfmt.F(p.UnrealizedPnL),
			trim(numfmt.F(p.Leverage)+"x", 5),
		}
		line := formatRow(row, widths)
		// Style 優先序：cursor 反白 > unmatched 橘 > PnL 紅綠
		switch {
		case i == m.cursor:
			line = "> " + line[2:]
			line = styles.Selected.Render(line)
		case unmatched[p.Symbol]:
			line = styles.WarnLegRow.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
		totalPnL += p.UnrealizedPnL
	}

	// Errors area
	if len(m.errors) > 0 {
		b.WriteString("\n")
		for name, err := range m.errors {
			b.WriteString(styles.ErrorText.Render(fmt.Sprintf("✗ %s: %v", name, err)))
			b.WriteString("\n")
		}
	}

	// Status line
	b.WriteString(strings.Repeat("─", sumWidth(widths)))
	b.WriteString("\n")
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

func formatRow(cols []string, widths []int) string {
	var b strings.Builder
	for i, c := range cols {
		w := widths[i]
		if len(c) > w {
			c = c[:w]
		}
		b.WriteString(c)
		if pad := w - len(c); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteByte(' ')
	}
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

func sumWidth(widths []int) int {
	s := 0
	for _, w := range widths {
		s += w + 1
	}
	return s
}

func trim(s string, w int) string {
	if len(s) > w {
		return s[:w]
	}
	return s
}

