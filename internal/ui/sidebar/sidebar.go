// Package sidebar 提供左側 tab 切換 list。
//
// 用 bubbles/v2/list；filter / status bar / pagination 全關掉，
// 只保留游標 + 渲染。鍵盤事件由 root app 過濾後再交給 list（避免 list
// 自帶的 "/" filter / "q" quit 等衝突）。
package sidebar

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourname/poscli/internal/ui/styles"
)

// Width 是 sidebar 的固定寬度（含左右 padding）。
const Width = 18

// item 是 sidebar 的一格。
type item struct {
	label string
}

func (i item) FilterValue() string { return i.label }

// delegate 控制每一格怎麼畫。
type delegate struct{}

func (delegate) Height() int                             { return 1 }
func (delegate) Spacing() int                            { return 0 }
func (delegate) Update(tea.Msg, *list.Model) tea.Cmd     { return nil }
func (d delegate) Render(w io.Writer, m list.Model, idx int, it list.Item) {
	i, ok := it.(item)
	if !ok {
		return
	}
	text := "  " + i.label
	if idx == m.Index() {
		// 活動 tab：accent 顏色 + 粗體 + 左側 marker
		text = "▶ " + i.label
		text = lipgloss.NewStyle().
			Bold(true).
			Foreground(styles.AccentColor).
			Render(text)
	} else {
		text = styles.Dim.Render(text)
	}
	_, _ = fmt.Fprint(w, text)
}

// Model 包 bubbles list。
type Model struct {
	list list.Model
}

// New 建立帶有 tab 名稱的 sidebar。
func New(tabs []string, height int) Model {
	items := make([]list.Item, 0, len(tabs))
	for _, t := range tabs {
		items = append(items, item{label: t})
	}
	l := list.New(items, delegate{}, Width, height)
	l.Title = "poscli"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.AccentColor).
		Padding(0, 1)
	return Model{list: l}
}

// Index 回傳目前活動 tab 的索引。
func (m Model) Index() int { return m.list.Index() }

// Select 設定活動 tab 的索引（給外部 1/2/3 快捷鍵用）。
func (m *Model) Select(i int) { m.list.Select(i) }

// SetSize 設 sidebar 高度。寬度固定為 Width。
func (m *Model) SetSize(_, height int) { m.list.SetSize(Width, height) }

// Update 把 KeyPress 轉為 list 的 cursor 移動。
// 只接受 up/down/k/j/home/end/pgup/pgdn 與 1/2/3；其他鍵透傳。
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "up", "k", "down", "j", "home", "end", "pgup", "pgdown":
			// allowed — falls through to list.Update
		default:
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View 渲染 sidebar；外圍框線 + accent 邊框。
func (m Model) View() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor).
		Padding(0, 0).
		Width(Width)
	return style.Render(strings.TrimRight(m.list.View(), "\n"))
}
