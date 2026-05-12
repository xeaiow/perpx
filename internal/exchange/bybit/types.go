package bybit

// envelope 是 Bybit V5 公用回應外殼。
type envelope struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  any    `json:"result"`
	Time    int64  `json:"time"`
}

// rawPositionList 對應 GET /v5/position/list 的 result。
type rawPositionList struct {
	List           []rawPosition `json:"list"`
	Category       string        `json:"category"`
	NextPageCursor string        `json:"nextPageCursor"`
}

type rawPosition struct {
	Symbol         string `json:"symbol"`
	Side           string `json:"side"`
	Size           string `json:"size"`
	PositionValue  string `json:"positionValue"`
	AvgPrice       string `json:"avgPrice"`
	MarkPrice      string `json:"markPrice"`
	Leverage       string `json:"leverage"`
	UnrealisedPnl  string `json:"unrealisedPnl"`
	TradeMode      int    `json:"tradeMode"`
	PositionIdx    int    `json:"positionIdx"`
	CreatedTime    string `json:"createdTime"`
	UpdatedTime    string `json:"updatedTime"`
}

// rawWalletBalance 對應 GET /v5/account/wallet-balance 的 result。
type rawWalletBalance struct {
	List []struct {
		AccountType string         `json:"accountType"`
		Coin        []rawCoinEntry `json:"coin"`
	} `json:"list"`
}

type rawCoinEntry struct {
	Coin                string `json:"coin"`
	WalletBalance       string `json:"walletBalance"`
	AvailableToWithdraw string `json:"availableToWithdraw"`
	UnrealisedPnl       string `json:"unrealisedPnl"`
	Equity              string `json:"equity"`
}

// rawClosedPnlList 對應 GET /v5/position/closed-pnl 的 result。
type rawClosedPnlList struct {
	List           []rawClosedPnl `json:"list"`
	Category       string         `json:"category"`
	NextPageCursor string         `json:"nextPageCursor"`
}

type rawClosedPnl struct {
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Qty           string `json:"qty"`
	OrderPrice    string `json:"orderPrice"`
	OrderType     string `json:"orderType"`
	ExecType      string `json:"execType"`
	ClosedSize    string `json:"closedSize"`
	CumEntryValue string `json:"cumEntryValue"`
	AvgEntryPrice string `json:"avgEntryPrice"`
	CumExitValue  string `json:"cumExitValue"`
	AvgExitPrice  string `json:"avgExitPrice"`
	ClosedPnl     string `json:"closedPnl"`
	FillCount     string `json:"fillCount"`
	Leverage      string `json:"leverage"`
	CreatedTime   string `json:"createdTime"`
	UpdatedTime   string `json:"updatedTime"`
}

// rawOrderCreateResult 對應 POST /v5/order/create 的 result。
type rawOrderCreateResult struct {
	OrderID     string `json:"orderId"`
	OrderLinkID string `json:"orderLinkId"`
}

// rawServerTime 對應 GET /v5/market/time 的 result。
type rawServerTime struct {
	TimeSecond string `json:"timeSecond"`
	TimeNano   string `json:"timeNano"`
}
