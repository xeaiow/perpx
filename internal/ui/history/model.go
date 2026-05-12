package history

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

// DefaultSince 預設拉 7 日內。
const DefaultSinceDuration = 7 * 24 * time.Hour

type Model struct {
	exs       map[string]exchange.Exchange
	items     []exchange.ClosedPosition
	errors    map[string]error
	lastFetch time.Time
	scroll    int
	filter    string // 空字串 = all
	width     int
	height    int
	loading   bool
}

func New(exs map[string]exchange.Exchange) Model {
	return Model{exs: exs, errors: map[string]error{}}
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
	case uimsg.HistoryFetchedMsg:
		m.loading = false
		m.items = sortByCloseDesc(msg.Items)
		m.errors = msg.Errors
		m.lastFetch = msg.At
	case tea.KeyPressMsg:
		switch msg.String() {
		case "j", "down":
			if m.scroll < max0(len(m.filtered())-1) {
				m.scroll++
			}
		case "k", "up":
			if m.scroll > 0 {
				m.scroll--
			}
		case "pgdown":
			m.scroll = min(len(m.filtered())-1, m.scroll+10)
			if m.scroll < 0 {
				m.scroll = 0
			}
		case "pgup":
			m.scroll = max0(m.scroll - 10)
		case "f":
			m.cycleFilter()
		case "r":
			m.loading = true
			return m, FetchCmd(m.exs, time.Now().Add(-DefaultSinceDuration))
		}
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
	// find current then advance
	idx := 0
	for i, name := range order {
		if name == m.filter {
			idx = i
			break
		}
	}
	m.filter = order[(idx+1)%len(order)]
	m.scroll = 0
}

func sortByCloseDesc(in []exchange.ClosedPosition) []exchange.ClosedPosition {
	out := append([]exchange.ClosedPosition(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CloseTime.After(out[j].CloseTime)
	})
	return out
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
