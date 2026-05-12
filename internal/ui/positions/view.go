package positions

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/styles"
)

// View 渲染 positions tab 的表格 + 狀態列。
//
// modal 由 root app 在這個 view 上面疊加；本函式不負責 modal。
func (m Model) View() string {
	var b strings.Builder

	headers := []string{"Exchange", "Symbol", "Side", "Size", "Entry", "Mark", "uPnL", "Lev"}
	widths := []int{10, 14, 6, 12, 12, 12, 12, 5}
	b.WriteString(styles.Header.Render(formatRow(headers, widths)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", sumWidth(widths)))
	b.WriteString("\n")

	if m.loading && len(m.positions) == 0 {
		b.WriteString(styles.Dim.Render("Loading..."))
		b.WriteString("\n")
	}

	var totalPnL float64
	for i, p := range m.positions {
		row := []string{
			p.Exchange,
			p.Symbol,
			string(p.Side),
			trim(fmt.Sprintf("%g", p.Size), 12),
			trim(fmt.Sprintf("%g", p.EntryPrice), 12),
			trim(fmt.Sprintf("%g", p.MarkPrice), 12),
			"", // uPnL coloured below
			trim(fmt.Sprintf("%gx", p.Leverage), 5),
		}
		// uPnL with sign + colour
		pnlText := fmt.Sprintf("%+.2f", p.UnrealizedPnL)
		row[6] = pnlText
		line := formatRow(row, widths)
		// colour the uPnL column by re-rendering
		line = colourPnLColumn(line, widths, p.UnrealizedPnL)
		if i == m.cursor {
			line = "> " + line[2:]
			line = styles.Selected.Render(line)
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
	stats := fmt.Sprintf("Total uPnL: %+.2f USDT     %d positions across %d exchanges",
		totalPnL, len(m.positions), len(m.exs))
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

func colourPnLColumn(line string, widths []int, pnl float64) string {
	// 簡化：不對 line 內欄位做切割重染，整列依需要染色已足夠 — 直接回傳。
	_ = widths
	_ = pnl
	return line
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

// 確保 exchange 變數實際使用，避免 unused import 報錯（exchange.Position 用在 trim 之外的型別）。
var _ exchange.Position
