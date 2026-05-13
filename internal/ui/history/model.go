package history

import (
	"sort"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/numfmt"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

// DefaultSince 預設拉 7 日內。
const DefaultSinceDuration = 7 * 24 * time.Hour

type Model struct {
	exs       map[string]exchange.Exchange
	items     []exchange.ClosedPosition
	errors    map[string]error
	lastFetch time.Time
	filter    string // 空字串 = all
	width     int
	height    int
	loading   bool

	tbl table.Model
}

func New(exs map[string]exchange.Exchange) Model {
	tbl := table.New(
		table.WithColumns(historyColumns()),
		table.WithFocused(true),
		table.WithHeight(20),
		table.WithWidth(120),
	)
	return Model{exs: exs, errors: map[string]error{}, tbl: tbl}
}

func historyColumns() []table.Column {
	return []table.Column{
		{Title: "Time", Width: 16},
		{Title: "Exchange", Width: 10},
		{Title: "Symbol", Width: 14},
		{Title: "Side", Width: 6},
		{Title: "Size", Width: 12},
		{Title: "Entry", Width: 12},
		{Title: "Exit", Width: 12},
		{Title: "PnL", Width: 12},
	}
}

func (m *Model) Init() tea.Cmd {
	m.loading = true
	return FetchCmd(m.exs, time.Now().Add(-DefaultSinceDuration))
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h := msg.Height - 8
		if h < 5 {
			h = 5
		}
		m.tbl.SetHeight(h)
		w := msg.Width - 20
		if w < 60 {
			w = 60
		}
		m.tbl.SetWidth(w)
	case uimsg.HistoryFetchedMsg:
		m.loading = false
		m.items = sortByCloseDesc(msg.Items)
		m.errors = msg.Errors
		m.lastFetch = msg.At
		m.tbl.SetRows(itemsToRows(m.filtered()))
	case tea.KeyPressMsg:
		switch msg.String() {
		case "f":
			m.cycleFilter()
			m.tbl.SetRows(itemsToRows(m.filtered()))
			return m, nil
		case "r":
			m.loading = true
			return m, FetchCmd(m.exs, time.Now().Add(-DefaultSinceDuration))
		}
		// 其餘鍵交給 bubbles table（up/down/j/k/home/end/pgup/pgdn）。
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) Refresh() (Model, tea.Cmd) {
	m.loading = true
	return m, FetchCmd(m.exs, time.Now().Add(-DefaultSinceDuration))
}

func (m Model) filtered() []exchange.ClosedPosition {
	if m.filter == "" {
		return m.items
	}
	out := make([]exchange.ClosedPosition, 0, len(m.items))
	for _, it := range m.items {
		if it.Exchange == m.filter {
			out = append(out, it)
		}
	}
	return out
}

func (m *Model) cycleFilter() {
	order := []string{""}
	seen := map[string]bool{"": true}
	for name := range m.exs {
		if !seen[name] {
			order = append(order, name)
			seen[name] = true
		}
	}
	sort.Strings(order[1:])
	idx := 0
	for i, name := range order {
		if name == m.filter {
			idx = i
			break
		}
	}
	m.filter = order[(idx+1)%len(order)]
}

// itemsToRows 把 ClosedPosition 轉成 table.Row。零值欄位顯示 —。
func itemsToRows(items []exchange.ClosedPosition) []table.Row {
	out := make([]table.Row, 0, len(items))
	for _, it := range items {
		timeStr := "—"
		if !it.CloseTime.IsZero() {
			timeStr = it.CloseTime.Local().Format("01-02 15:04")
		}
		out = append(out, table.Row{
			timeStr,
			it.Exchange,
			it.Symbol,
			string(it.Side),
			formatOrDash(it.Size),
			formatOrDash(it.EntryPrice),
			formatOrDash(it.ExitPrice),
			numfmt.F(it.RealizedPnL),
		})
	}
	return out
}

func formatOrDash(v float64) string {
	if v == 0 {
		return "—"
	}
	return numfmt.F(v)
}

func sortByCloseDesc(in []exchange.ClosedPosition) []exchange.ClosedPosition {
	out := append([]exchange.ClosedPosition(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CloseTime.After(out[j].CloseTime)
	})
	return out
}
