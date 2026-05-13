package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yourname/poscli/internal/closelog"
	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/accounts"
	"github.com/yourname/poscli/internal/ui/confirm"
	"github.com/yourname/poscli/internal/ui/history"
	"github.com/yourname/poscli/internal/ui/positions"
	"github.com/yourname/poscli/internal/ui/sidebar"
	"github.com/yourname/poscli/internal/ui/styles"
)

// Tab 索引。
const (
	TabPositions = 0
	TabHistory   = 1
	TabAccounts  = 2
)

type App struct {
	tab int

	positions positions.Model
	history   history.Model
	accounts  accounts.Model
	sidebar   sidebar.Model

	exs map[string]exchange.Exchange

	showHelp bool

	historyInited  bool
	accountsInited bool

	toast      string
	toastError bool

	width, height int
}

// NewFromMap 用 string-keyed map 建立 App。
func NewFromMap(exs map[string]exchange.Exchange) *App {
	return &App{
		exs:       exs,
		positions: positions.New(exs),
		history:   history.New(exs),
		accounts:  accounts.New(exs),
		sidebar:   sidebar.New([]string{"Positions", "History", "Accounts"}, 20),
	}
}

// Init 啟動 positions tab 的初始 fetch；history/accounts 在切過去時才啟動。
func (a *App) Init() tea.Cmd {
	return a.positions.Init()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.sidebar.SetSize(sidebar.Width, msg.Height-2)
	case tea.MouseWheelMsg:
		// 滑鼠滾輪 sidebar 切換
		switch msg.Button {
		case tea.MouseWheelUp:
			if a.tab > 0 {
				a.tab--
				a.sidebar.Select(a.tab)
				return a, a.lazyInit()
			}
		case tea.MouseWheelDown:
			if a.tab < 2 {
				a.tab++
				a.sidebar.Select(a.tab)
				return a, a.lazyInit()
			}
		}
		return a, nil
	case tea.KeyPressMsg:
		// 全域 quit / help
		switch msg.String() {
		case "q", "ctrl+c":
			if a.positions.ConfirmOpen && !a.positions.ConfirmInFlight {
				if t := a.positions.ConfirmTarget; t != nil {
					closelog.Cancelled(logFields(*t))
				}
				a.positions.SetConfirm(false)
				return a, nil
			}
			return a, tea.Quit
		case "?":
			a.showHelp = !a.showHelp
			return a, nil
		case "tab":
			a.tab = (a.tab + 1) % 3
			a.sidebar.Select(a.tab)
			return a, a.lazyInit()
		case "shift+tab":
			a.tab = (a.tab + 2) % 3
			a.sidebar.Select(a.tab)
			return a, a.lazyInit()
		case "1":
			a.tab = TabPositions
			a.sidebar.Select(a.tab)
			return a, nil
		case "2":
			a.tab = TabHistory
			a.sidebar.Select(a.tab)
			return a, a.lazyInit()
		case "3":
			a.tab = TabAccounts
			a.sidebar.Select(a.tab)
			return a, a.lazyInit()
		}

		// modal 鍵盤
		if a.tab == TabPositions && a.positions.ConfirmOpen {
			switch msg.String() {
			case "y":
				if a.positions.ConfirmInFlight {
					return a, nil
				}
				p := a.positions.ConfirmTarget
				if p == nil {
					return a, nil
				}
				a.positions.ConfirmInFlight = true
				return a, closeCmd(a.exs, *p)
			case "n", "esc":
				if a.positions.ConfirmInFlight {
					return a, nil
				}
				if t := a.positions.ConfirmTarget; t != nil {
					closelog.Cancelled(logFields(*t))
				}
				a.positions.SetConfirm(false)
				return a, nil
			}
			return a, nil
		}

		if a.tab == TabPositions && msg.String() == "x" {
			if a.positions.SelectedPosition() != nil {
				a.positions.SetConfirm(true)
			}
			return a, nil
		}

	case CloseResultMsg:
		a.positions.ConfirmInFlight = false
		a.positions.SetConfirm(false)
		if msg.Err != nil {
			a.toast = fmt.Sprintf("✗ Failed to close %s:%s — %v", msg.Exchange, msg.Symbol, msg.Err)
			a.toastError = true
		} else {
			a.toast = fmt.Sprintf("✓ Closed %s:%s", msg.Exchange, msg.Symbol)
			a.toastError = false
		}
		return a, tea.Batch(toastClearAfter(3*time.Second), positions.FetchCmd(a.exs))

	case ToastClearMsg:
		a.toast = ""
		return a, nil
	}

	// 分發給所有 tab，讓背景 fetch 完成的 msg 抵達
	var cmd tea.Cmd
	a.positions, cmd = a.positions.Update(msg)
	if cmd != nil {
		return a, cmd
	}
	a.history, cmd = a.history.Update(msg)
	if cmd != nil {
		return a, cmd
	}
	a.accounts, cmd = a.accounts.Update(msg)
	return a, cmd
}

func (a *App) View() tea.View {
	var mainB strings.Builder
	switch a.tab {
	case TabPositions:
		mainB.WriteString(a.positions.View())
	case TabHistory:
		mainB.WriteString(a.history.View())
	case TabAccounts:
		mainB.WriteString(a.accounts.View())
	}
	if a.toast != "" {
		mainB.WriteString("\n")
		if a.toastError {
			mainB.WriteString(styles.ErrorText.Render(a.toast))
		} else {
			mainB.WriteString(styles.OKText.Render(a.toast))
		}
	}
	if a.showHelp {
		mainB.WriteString("\n")
		mainB.WriteString(renderHelp())
	}
	if a.tab == TabPositions && a.positions.ConfirmOpen && a.positions.ConfirmTarget != nil {
		mainB.WriteString("\n\n")
		mainB.WriteString(confirm.Render(*a.positions.ConfirmTarget, a.positions.ConfirmInFlight))
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, a.sidebar.View(), mainB.String())
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

// lazyInit：切到 history/accounts 時，若尚未初始化，觸發其 fetch。
func (a *App) lazyInit() tea.Cmd {
	switch a.tab {
	case TabHistory:
		if !a.historyInited {
			a.historyInited = true
			return a.history.Init()
		}
	case TabAccounts:
		if !a.accountsInited {
			a.accountsInited = true
			return a.accounts.Init()
		}
	}
	return nil
}

func renderHelp() string {
	lines := []string{
		"Global keys:",
		"  tab/shift+tab   switch tab",
		"  1/2/3           jump to tab",
		"  r               refresh current tab",
		"  q / ctrl+c      quit",
		"  ?               toggle this help",
		"",
		"Positions tab:",
		"  ↑↓ / jk         move selection",
		"  home/g / end/G  first / last row",
		"  x               close selected (confirm with y)",
		"",
		"History tab:",
		"  ↑↓ / jk         scroll",
		"  pgup / pgdn     page",
		"  f               cycle exchange filter",
	}
	return styles.Modal.Render(strings.Join(lines, "\n"))
}

func closeCmd(exs map[string]exchange.Exchange, p exchange.Position) tea.Cmd {
	return func() tea.Msg {
		fields := logFields(p)
		closelog.Requested(fields)

		ex, ok := exs[p.Exchange]
		if !ok {
			err := fmt.Errorf("unknown exchange %q", p.Exchange)
			closelog.Failed(fields, err, 0)
			return CloseResultMsg{Exchange: p.Exchange, Symbol: p.Symbol, Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		start := time.Now()
		res, err := ex.ClosePosition(ctx, exchange.CloseRequest{
			Symbol:     p.RawSymbol,
			Side:       p.Side,
			RawSide:    p.RawSide,
			Size:       p.Size,
			MarginMode: p.MarginMode,
		})
		latency := time.Since(start)
		if err != nil {
			closelog.Failed(fields, err, latency)
		} else {
			closelog.Completed(fields, res.OrderID, latency)
		}
		return CloseResultMsg{Exchange: p.Exchange, Symbol: p.Symbol, Result: res, Err: err}
	}
}

func logFields(p exchange.Position) closelog.Fields {
	return closelog.Fields{
		Exchange:   p.Exchange,
		Symbol:     p.Symbol,
		RawSymbol:  p.RawSymbol,
		Side:       string(p.Side),
		Size:       p.Size,
		CoinSize:   p.CoinSize,
		EntryPrice: p.EntryPrice,
		MarkPrice:  p.MarkPrice,
		UPnL:       p.UnrealizedPnL,
		MarginMode: p.MarginMode,
	}
}

func toastClearAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg { return ToastClearMsg{} })
}
