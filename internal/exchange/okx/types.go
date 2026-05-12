package okx

type envelope struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type rawPosition struct {
	InstID      string `json:"instId"`
	InstType    string `json:"instType"`
	MgnMode     string `json:"mgnMode"`
	PosSide     string `json:"posSide"`
	Pos         string `json:"pos"`
	AvgPx       string `json:"avgPx"`
	MarkPx      string `json:"markPx"`
	Upl         string `json:"upl"`
	Lever       string `json:"lever"`
	NotionalUsd string `json:"notionalUsd"`
	Ccy         string `json:"ccy"`
	UTime       string `json:"uTime"`
	CTime       string `json:"cTime"`
}

type rawBalanceData struct {
	TotalEq string             `json:"totalEq"`
	Details []rawBalanceDetail `json:"details"`
}

type rawBalanceDetail struct {
	Ccy     string `json:"ccy"`
	AvailBal string `json:"availBal"`
	AvailEq string `json:"availEq"`
	CashBal string `json:"cashBal"`
	Eq      string `json:"eq"`
}

type rawClosedPosition struct {
	InstID      string `json:"instId"`
	PosSide     string `json:"posSide"`
	OpenAvgPx   string `json:"openAvgPx"`
	CloseAvgPx  string `json:"closeAvgPx"`
	RealizedPnl string `json:"realizedPnl"`
	Pnl         string `json:"pnl"`
	OpenTime    string `json:"openTime"`
	UTime       string `json:"uTime"`
	CloseTotal  string `json:"closeTotalPos"`
}

type rawCloseResult struct {
	InstID  string `json:"instId"`
	PosSide string `json:"posSide"`
	ClOrdID string `json:"clOrdId"`
}

type rawServerTime struct {
	Ts string `json:"ts"`
}
