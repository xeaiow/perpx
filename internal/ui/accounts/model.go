package accounts

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/styles"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

type Model struct {
	exs       map[string]exchange.Exchange
	equity    map[string]float64
	errors    map[string]error
	lastFetch time.Time
	loading   bool
}

func New(exs map[string]exchange.Exchange) Model {
	return Model{exs: exs, equity: map[string]float64{}, errors: map[string]error{}}
}

func (m *Model) Init() tea.Cmd {
	m.loading = true
	return FetchCmd(m.exs)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case uimsg.AccountsFetchedMsg:
		m.loading = false
		m.equity = msg.Equity
		m.errors = msg.Errors
		m.lastFetch = msg.At
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			m.loading = true
			return m, FetchCmd(m.exs)
		}
	}
	return m, nil
}

func (m Model) Refresh() (Model, tea.Cmd) {
	m.loading = true
	return m, FetchCmd(m.exs)
}

func (m Model) View() string {
	var b strings.Builder
	headers := []string{"Exchange", "Equity (USDT)"}
	widths := []int{12, 20}
	b.WriteString(styles.Header.Render(formatRow(headers, widths)))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", sumWidth(widths)))
	b.WriteString("\n")
	if m.loading && len(m.equity) == 0 {
		b.WriteString(styles.Dim.Render("Loading..."))
		b.WriteString("\n")
	}

	names := make([]string, 0, len(m.exs))
	for n := range m.exs {
		names = append(names, n)
	}
	sort.Strings(names)
	var total float64
	for _, n := range names {
		v, ok := m.equity[n]
		var display string
		if ok {
			display = fmt.Sprintf("%.2f", v)
			total += v
		} else if _, hasErr := m.errors[n]; hasErr {
			display = "—"
		} else {
			display = "..."
		}
		b.WriteString(formatRow([]string{n, display}, widths))
		b.WriteString("\n")
	}
	b.WriteString(strings.Repeat("─", sumWidth(widths)))
	b.WriteString("\n")
	b.WriteString(styles.Header.Render(formatRow([]string{"TOTAL", fmt.Sprintf("%.2f", total)}, widths)))
	b.WriteString("\n")

	if len(m.errors) > 0 {
		b.WriteString("\n")
		for name, err := range m.errors {
			b.WriteString(styles.ErrorText.Render(fmt.Sprintf("✗ %s: %v", name, err)))
			b.WriteString("\n")
		}
	}

	if !m.lastFetch.IsZero() {
		ago := time.Since(m.lastFetch).Round(time.Second)
		b.WriteString(styles.Dim.Render(fmt.Sprintf("Updated %s ago", ago)))
		b.WriteString("\n")
	}
	b.WriteString(styles.Dim.Render("[r] refresh  [tab] switch view"))
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

func sumWidth(widths []int) int {
	s := 0
	for _, w := range widths {
		s += w + 1
	}
	return s
}
