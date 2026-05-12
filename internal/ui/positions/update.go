package positions

import (
	"sort"

	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/uimsg"
)

// Init 啟動時先觸發一次 fetch。
//
// 注意：v2 介面要求 Init 只回 Cmd，loading 旗標改由 fetch 完成前其他訊息判斷。
func (m *Model) Init() tea.Cmd {
	m.loading = true
	return FetchCmd(m.exs)
}

// Update 處理鍵盤與訊息。
//
// 注意：close-position 流程的 modal 鍵與提交在 M6 接入；此處只負責 navigation + refresh。
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case uimsg.PositionsFetchedMsg:
		m.loading = false
		m.positions = sortPositions(msg.Positions)
		m.errors = msg.Errors
		m.lastFetch = msg.At
		if m.cursor >= len(m.positions) {
			m.cursor = max0(len(m.positions) - 1)
		}
	case tea.KeyPressMsg:
		// modal 開啟時把鍵盤交給 app 處理（app.go 會 inspect ConfirmOpen），這裡略過。
		if m.ConfirmOpen {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.positions)-1 {
				m.cursor++
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = max0(len(m.positions) - 1)
		case "r":
			m.loading = true
			return m, FetchCmd(m.exs)
		}
	}
	return m, nil
}

// Refresh 觸發外部刷新（給 app.go 切 tab 後使用）。
func (m Model) Refresh() (Model, tea.Cmd) {
	m.loading = true
	return m, FetchCmd(m.exs)
}

// SetConfirm 把選取倉位送進 close 確認 modal（M6 使用）。
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
