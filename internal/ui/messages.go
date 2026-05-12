// Package ui 是 Bubble Tea TUI 的根組合。每個 tab 各自為 sub-package。
//
// 跨套件 msg 型別在 internal/ui/uimsg；此檔 re-export 供 ui 套件內部簡寫。
package ui

import "github.com/yourname/poscli/internal/ui/uimsg"

type (
	PositionsFetchedMsg = uimsg.PositionsFetchedMsg
	HistoryFetchedMsg   = uimsg.HistoryFetchedMsg
	AccountsFetchedMsg  = uimsg.AccountsFetchedMsg
	CloseResultMsg      = uimsg.CloseResultMsg
	SwitchTabMsg        = uimsg.SwitchTabMsg
	ToastClearMsg       = uimsg.ToastClearMsg
)
