// Package styles 集中定義 TUI 樣式，方便整體調色與比對。
package styles

import "charm.land/lipgloss/v2"

var (
	BorderColor = lipgloss.Color("#444444")
	AccentColor = lipgloss.Color("#00B5D8")
	PnlPositive = lipgloss.Color("#0AC18E")
	PnlNegative = lipgloss.Color("#F03A47")
	DimColor    = lipgloss.Color("#888888")
	WarnOrange  = lipgloss.Color("#FF8C00") // 用於標示異常（如 unmatched-leg）的橘色

	TabActive   = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(AccentColor).Padding(0, 1)
	TabInactive = lipgloss.NewStyle().Foreground(DimColor).Padding(0, 1)

	Header = lipgloss.NewStyle().Bold(true).Foreground(AccentColor)

	Selected = lipgloss.NewStyle().Bold(true).Reverse(true)
	Dim      = lipgloss.NewStyle().Foreground(DimColor)

	ErrorText  = lipgloss.NewStyle().Foreground(PnlNegative).Bold(true)
	OKText     = lipgloss.NewStyle().Foreground(PnlPositive)
	WarnLegRow = lipgloss.NewStyle().Foreground(WarnOrange)

	Modal = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(AccentColor).
		Padding(1, 2)
)

// Pnl 為依數值正負取色的 style。
func Pnl(v float64) lipgloss.Style {
	if v >= 0 {
		return lipgloss.NewStyle().Foreground(PnlPositive)
	}
	return lipgloss.NewStyle().Foreground(PnlNegative)
}
