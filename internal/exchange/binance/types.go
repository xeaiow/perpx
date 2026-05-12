package binance

type rawErr struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type rawPosition struct {
	Symbol           string `json:"symbol"`
	PositionSide     string `json:"positionSide"` // BOTH / LONG / SHORT
	PositionAmt      string `json:"positionAmt"`
	EntryPrice       string `json:"entryPrice"`
	MarkPrice        string `json:"markPrice"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	Leverage         string `json:"leverage"`
	MarginType       string `json:"marginType"`
	Notional         string `json:"notional"`
	MarginAsset      string `json:"marginAsset"`
	UpdateTime       int64  `json:"updateTime"`
}

type rawBalanceEntry struct {
	AccountAlias     string `json:"accountAlias"`
	Asset            string `json:"asset"`
	Balance          string `json:"balance"`
	CrossWalletBal   string `json:"crossWalletBalance"`
	AvailableBalance string `json:"availableBalance"`
	UpdateTime       int64  `json:"updateTime"`
}

type rawIncome struct {
	Symbol     string `json:"symbol"`
	IncomeType string `json:"incomeType"`
	Income     string `json:"income"`
	Asset      string `json:"asset"`
	Info       string `json:"info"`
	Time       int64  `json:"time"`
	TranID     int64  `json:"tranId"`
	TradeID    string `json:"tradeId"`
}

type rawOrderResp struct {
	OrderID int64  `json:"orderId"`
	Symbol  string `json:"symbol"`
	Status  string `json:"status"`
}

type rawTime struct {
	ServerTime int64 `json:"serverTime"`
}
