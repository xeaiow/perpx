// Package uimsg 是 TUI 跨套件共用的 tea.Msg 型別。
// 拉到獨立 package 是為了避免 internal/ui 與 internal/ui/<tab> 之間的 import cycle。
package uimsg

import (
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

type PositionsFetchedMsg struct {
	Positions []exchange.Position
	Errors    map[string]error
	At        time.Time
}

type HistoryFetchedMsg struct {
	Items  []exchange.ClosedPosition
	Errors map[string]error
	At     time.Time
}

type AccountsFetchedMsg struct {
	Equity map[string]float64
	Errors map[string]error
	At     time.Time
}

type CloseResultMsg struct {
	Exchange string
	Symbol   string
	Result   exchange.CloseResult
	Err      error
}

type SwitchTabMsg int

type ToastClearMsg struct{}
