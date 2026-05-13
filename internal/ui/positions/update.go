package positions

import (
	"sort"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/numfmt"
	"github.com/yourname/poscli/internal/ui/styles"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

// Init 啟動時先觸發一次 fetch。
func (m *Model) Init() tea.Cmd {
	m.loading = true
	return FetchCmd(m.exs)
}

// Update 處理鍵盤與訊息。
//
// 表格 cursor / 鍵盤交給 bubbles/table 內建 Update 處理；
// modal 開啟時不把鍵交給 table，避免 j/k 移動游標。
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// 預留一些行給 header + status line + error area。
		h := msg.Height - 8
		if h < 5 {
			h = 5
		}
		m.tbl.SetHeight(h)
		w := msg.Width - 20 // 留給未來 sidebar 用的空間
		if w < 60 {
			w = 60
		}
		m.tbl.SetWidth(w)
	case uimsg.PositionsFetchedMsg:
		m.loading = false
		m.positions = sortPositions(msg.Positions)
		m.errors = msg.Errors
		m.lastFetch = msg.At
		m.tbl.SetRows(positionsToRows(m.positions))
		if m.tbl.Cursor() >= len(m.positions) {
			m.tbl.SetCursor(max0(len(m.positions) - 1))
		}
	case tea.KeyPressMsg:
		if m.ConfirmOpen {
			return m, nil
		}
		if msg.String() == "r" {
			m.loading = true
			return m, FetchCmd(m.exs)
		}
		// 把鍵盤事件交給 bubbles table 處理（up/down/j/k/home/end/pgup/pgdn）。
		var cmd tea.Cmd
		m.tbl, cmd = m.tbl.Update(msg)
		return m, cmd
	}
	return m, nil
}

// Refresh 觸發外部刷新（給 app.go 切 tab 後使用）。
func (m Model) Refresh() (Model, tea.Cmd) {
	m.loading = true
	return m, FetchCmd(m.exs)
}

// SetConfirm 把選取倉位送進 close 確認 modal。
func (m *Model) SetConfirm(open bool) {
	if !open {
		m.ConfirmOpen = false
		m.ConfirmTarget = nil
		return
	}
	p := m.SelectedPosition()
	if p == nil {
		return
	}
	m.ConfirmOpen = true
	m.ConfirmTarget = p
}

// positionsToRows 把每個 Position 轉成 table.Row（[]string）。
// 對 unmatched 倉位，把每個 cell 字串都用 lipgloss 橘色 wrap，
// 這樣 bubbles table 渲染時 ANSI 已嵌入、整列顯示橘色。
// （cursor 反白由 table 自己處理；ANSI 與反白疊合終端會自動處理。）
func positionsToRows(ps []exchange.Position) []table.Row {
	unmatched := unmatchedSymbols(ps)
	out := make([]table.Row, 0, len(ps))
	for _, p := range ps {
		coinText := "—"
		if p.CoinSize > 0 {
			coinText = numfmt.F(p.CoinSize)
		}
		cells := []string{
			p.Exchange,
			p.Symbol,
			string(p.Side),
			numfmt.F(p.Size),
			coinText,
			numfmt.F(p.EntryPrice),
			numfmt.F(p.MarkPrice),
			numfmt.F(p.UnrealizedPnL),
			numfmt.F(p.Leverage) + "x",
		}
		if unmatched[p.Symbol] {
			for i, c := range cells {
				cells[i] = styles.WarnLegRow.Render(c)
			}
		}
		out = append(out, table.Row(cells))
	}
	return out
}

func sortPositions(in []exchange.Position) []exchange.Position {
	out := append([]exchange.Position(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Exchange != out[j].Exchange {
			return out[i].Exchange < out[j].Exchange
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
