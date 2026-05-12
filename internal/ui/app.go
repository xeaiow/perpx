package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/accounts"
	"github.com/yourname/poscli/internal/ui/confirm"
	"github.com/yourname/poscli/internal/ui/history"
	"github.com/yourname/poscli/internal/ui/positions"
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

	exs map[string]exchange.Exchange

	showHelp bool

	historyInited  bool
	accountsInited bool

	toast      string
	toastError bool
}

// NewFromMap 用 string-keyed map 建立 App。
func NewFromMap(exs map[string]exchange.Exchange) *App {
	return &App{
		exs:       exs,
		positions: positions.New(exs),
		history:   history.New(exs),
		accounts:  accounts.New(exs),
	}
}

// Init 啟動 positions tab 的初始 fetch；history/accounts 在切過去時才啟動。
func (a *App) Init() tea.Cmd {
	return a.positions.Init()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		// 全域 quit / help
		switch msg.String() {
		case "q", "ctrl+c":
			if a.positions.ConfirmOpen && !a.positions.ConfirmInFlight {
				a.positions.SetConfirm(false)
				return a, nil
			}
			return a, tea.Quit
		case "?":
			a.showHelp = !a.showHelp
			return a, nil
		case "tab":
			a.tab = (a.tab + 1) % 3
			return a, a.lazyInit()
		case "shift+tab":
			a.tab = (a.tab + 2) % 3
			return a, a.lazyInit()
		case "1":
			a.tab = TabPositions
			return a, nil
		case "2":
			a.tab = TabHistory
			return a, a.lazyInit()
		case "3":
			a.tab = TabAccounts
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
	var b strings.Builder
	b.WriteString(renderTabs(a.tab))
	b.WriteString("\n")
	switch a.tab {
	case TabPositions:
		b.WriteString(a.positions.View())
	case TabHistory:
		b.WriteString(a.history.View())
	case TabAccounts:
		b.WriteString(a.accounts.View())
	}
	if a.toast != "" {
		b.WriteString("\n")
		if a.toastError {
			b.WriteString(styles.ErrorText.Render(a.toast))
		} else {
			b.WriteString(styles.OKText.Render(a.toast))
		}
	}
	if a.showHelp {
		b.WriteString("\n")
		b.WriteString(renderHelp())
	}
	if a.tab == TabPositions && a.positions.ConfirmOpen && a.positions.ConfirmTarget != nil {
		b.WriteString("\n\n")
		b.WriteString(confirm.Render(*a.positions.ConfirmTarget, a.positions.ConfirmInFlight))
	}
	v := tea.NewView(b.String())
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

func renderTabs(active int) string {
	names := []string{"Positions", "History", "Accounts"}
	var b strings.Builder
	b.WriteString(styles.Header.Render("poscli "))
	for i, n := range names {
		if i == active {
			b.WriteString(styles.TabActive.Render("[ " + n + " ]"))
		} else {
			b.WriteString(styles.TabInactive.Render(n))
		}
	}
	return b.String()
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
		ex, ok := exs[p.Exchange]
		if !ok {
			return CloseResultMsg{Exchange: p.Exchange, Symbol: p.Symbol, Err: fmt.Errorf("unknown exchange %q", p.Exchange)}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := ex.ClosePosition(ctx, exchange.CloseRequest{
			Symbol:     p.RawSymbol,
			Side:       p.Side,
			Size:       p.Size,
			MarginMode: p.MarginMode,
		})
		return CloseResultMsg{Exchange: p.Exchange, Symbol: p.Symbol, Result: res, Err: err}
	}
}

func toastClearAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg { return ToastClearMsg{} })
}
