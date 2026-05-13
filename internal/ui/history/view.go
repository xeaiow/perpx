package history

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourname/poscli/internal/ui/styles"
)

func (m Model) View() string {
	var b strings.Builder
	items := m.filtered()

	b.WriteString(m.tbl.View())
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

	if len(m.errors) > 0 {
		for name, err := range m.errors {
			b.WriteString(styles.ErrorText.Render(fmt.Sprintf("✗ %s: %v", name, err)))
			b.WriteString("\n")
		}
	}

	b.WriteString(styles.Dim.Render(fmt.Sprintf("Filter: %s  |  %d records", filterDisplay, len(items))))
	b.WriteString("\n")
	if !m.lastFetch.IsZero() {
		ago := time.Since(m.lastFetch).Round(time.Second)
		b.WriteString(styles.Dim.Render(fmt.Sprintf("Updated %s ago", ago)))
		b.WriteString("\n")
	}
	b.WriteString(styles.Dim.Render("[↑↓/jk] scroll  [pgup/pgdn] page  [f] cycle filter  [r] refresh"))
	return b.String()
}
