package bitget

type envelope struct {
	Code        string `json:"code"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
	Data        any    `json:"data"`
}

type rawPosition struct {
	Symbol           string `json:"symbol"`
	MarginCoin       string `json:"marginCoin"`
	HoldSide         string `json:"holdSide"`
	Available        string `json:"available"`
	Total            string `json:"total"`
	Leverage         string `json:"leverage"`
	OpenPriceAvg     string `json:"openPriceAvg"`
	MarginMode       string `json:"marginMode"`
	PosMode          string `json:"posMode"`
	UnrealizedPL     string `json:"unrealizedPL"`
	LiquidationPrice string `json:"liquidationPrice"`
	MarkPrice        string `json:"markPrice"`
	CTime            string `json:"cTime"`
	UTime            string `json:"uTime"`
}

type rawAccount struct {
	MarginCoin           string `json:"marginCoin"`
	Available            string `json:"available"`
	CrossedMaxAvailable  string `json:"crossedMaxAvailable"`
	IsolatedMaxAvailable string `json:"isolatedMaxAvailable"`
	AccountEquity        string `json:"accountEquity"`
	UsdtEquity           string `json:"usdtEquity"`
}

type rawHistoryWrapper struct {
	List  []rawHistoryItem `json:"list"`
	EndID string           `json:"endId"`
}

type rawHistoryItem struct {
	PositionID    string `json:"positionId"`
	MarginCoin    string `json:"marginCoin"`
	Symbol        string `json:"symbol"`
	HoldSide      string `json:"holdSide"`
	OpenAvgPrice  string `json:"openAvgPrice"`
	CloseAvgPrice string `json:"closeAvgPrice"`
	MarginMode    string `json:"marginMode"`
	OpenTotalPos  string `json:"openTotalPos"`
	CloseTotalPos string `json:"closeTotalPos"`
	Pnl           string `json:"pnl"`
	NetProfit     string `json:"netProfit"`
	CTime         string `json:"ctime"`
	UTime         string `json:"utime"`
}

type rawCloseResult struct {
	SuccessList []rawCloseEntry `json:"successList"`
	FailureList []rawCloseEntry `json:"failureList"`
}

type rawCloseEntry struct {
	OrderID   string `json:"orderId"`
	ClientOid string `json:"clientOid"`
	Symbol    string `json:"symbol"`
	ErrMsg    string `json:"errorMsg"`
	ErrCode   string `json:"errorCode"`
}

type rawServerTime struct {
	ServerTime string `json:"serverTime"`
}
