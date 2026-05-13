package positions

import (
	"time"

	"charm.land/bubbles/v2/table"

	"github.com/yourname/poscli/internal/exchange"
)

// Model 是 Positions tab 的狀態。
type Model struct {
	exs map[string]exchange.Exchange

	positions []exchange.Position
	errors    map[string]error
	lastFetch time.Time

	tbl   table.Model
	width  int
	height int

	loading bool
	// confirmOpen 給 M6（close 確認 modal）開關用。
	ConfirmOpen     bool
	ConfirmTarget   *exchange.Position
	ConfirmInFlight bool

	// Toast 顯示 close-position 結果（成功/錯誤）；空字串=隱藏。
	Toast      string
	ToastError bool
}

// New 建立 positions Model。
func New(exs map[string]exchange.Exchange) Model {
	tbl := table.New(
		table.WithColumns(positionsColumns()),
		table.WithFocused(true),
		table.WithHeight(20),
		table.WithWidth(120), // 預設寬；WindowSizeMsg 來時會覆寫
	)
	return Model{exs: exs, errors: map[string]error{}, tbl: tbl}
}

// positionsColumns 是 bubbles table 的欄位定義。寬度跟原本手刻的相近。
func positionsColumns() []table.Column {
	return []table.Column{
		{Title: "Exchange", Width: 10},
		{Title: "Symbol", Width: 14},
		{Title: "Side", Width: 6},
		{Title: "Size", Width: 12},
		{Title: "Coin", Width: 12},
		{Title: "Entry", Width: 12},
		{Title: "Mark", Width: 12},
		{Title: "uPnL", Width: 12},
		{Title: "Lev", Width: 5},
	}
}

// Positions 回傳當前快取的倉位（給其他 tab/Accounts 重用）。
func (m Model) Positions() []exchange.Position { return m.positions }

// Errors 回傳每個交易所的最後一次錯誤。
func (m Model) Errors() map[string]error { return m.errors }

// SelectedPosition 回傳目前選取列；無資料時為 nil。
func (m *Model) SelectedPosition() *exchange.Position {
	i := m.tbl.Cursor()
	if i < 0 || i >= len(m.positions) {
		return nil
	}
	return &m.positions[i]
}

// Exchanges 把目前已知交易所 map 傳出來（給 close cmd 拿到對應 adapter）。
func (m Model) Exchanges() map[string]exchange.Exchange { return m.exs }
