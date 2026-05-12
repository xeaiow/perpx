package history

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourname/poscli/internal/ui/styles"
)

const pageSize = 20

func (m Model) View() string {
	var b strings.Builder
	items := m.filtered()

	headers := []string{"Time", "Exchange", "Symbol", "Side", "Size", "Entry", "Exit", "PnL"}
	widths := []int{16, 10, 14, 6, 12, 12, 12, 12}
	b.WriteString(styles.Header.Render(formatRow(headers, widths)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", sumWidth(widths)))
	b.WriteString("\n")

	filterDisplay := m.filter
	if filterDisplay == "" {
		filterDisplay = "(all)"
	}

	if m.loading && len(items) == 0 {
		b.WriteString(styles.Dim.Render("Loading..."))
		b.WriteString("\n")
	}
	if len(items) == 0 && !m.loading {
		b.WriteString(styles.Dim.Render("No history in the last 7 days"))
		b.WriteString("\n")
	}

	start := m.scroll
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	for _, it := range items[start:end] {
		row := []string{
			it.CloseTime.Local().Format("01-02 15:04"),
			it.Exchange,
			it.Symbol,
			string(it.Side),
			formatOrDash(it.Size),
			formatOrDash(it.EntryPrice),
			formatOrDash(it.ExitPrice),
			fmt.Sprintf("%+.2f", it.RealizedPnL),
		}
		if it.CloseTime.IsZero() {
			row[0] = "—"
		}
		b.WriteString(styles.Pnl(it.RealizedPnL).Render(formatRow(row, widths)))
		b.WriteString("\n")
	}

	if len(m.errors) > 0 {
		b.WriteString("\n")
		for name, err := range m.errors {
			b.WriteString(styles.ErrorText.Render(fmt.Sprintf("✗ %s: %v", name, err)))
			b.WriteString("\n")
		}
	}

	b.WriteString(strings.Repeat("─", sumWidth(widths)))
	b.WriteString("\n")
	b.WriteString(styles.Dim.Render(fmt.Sprintf("Filter: %s  |  %d records  |  showing %d–%d", filterDisplay, len(items), start, end)))
	b.WriteString("\n")
	if !m.lastFetch.IsZero() {
		ago := time.Since(m.lastFetch).Round(time.Second)
		b.WriteString(styles.Dim.Render(fmt.Sprintf("Updated %s ago", ago)))
		b.WriteString("\n")
	}
	b.WriteString(styles.Dim.Render("[↑↓/jk] scroll  [pgup/pgdn] page  [f] cycle filter  [r] refresh"))
	return b.String()
}

func formatOrDash(v float64) string {
	if v == 0 {
		return "—"
	}
	return fmt.Sprintf("%g", v)
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

func sumWidth(widths []int) int {
	s := 0
	for _, w := range widths {
		s += w + 1
	}
	return s
}
