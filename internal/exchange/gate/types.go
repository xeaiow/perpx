package gate

type rawPosition struct {
	User               int     `json:"user"`
	Contract           string  `json:"contract"`
	Size               float64 `json:"size"` // 注意：Gate 是數字而非字串
	Leverage           string  `json:"leverage"`
	LeverageMax        string  `json:"leverage_max"`
	CrossLeverageLimit string  `json:"cross_leverage_limit"` // 跨倉模式下 leverage="0"，實際倍率在這裡
	Value              string  `json:"value"`
	Margin             string  `json:"margin"`
	EntryPrice         string  `json:"entry_price"`
	LiqPrice           string  `json:"liq_price"`
	MarkPrice          string  `json:"mark_price"`
	UnrealisedPnl      string  `json:"unrealised_pnl"`
	RealisedPnl        string  `json:"realised_pnl"`
	Mode               string  `json:"mode"` // single / dual_long / dual_short
}

type rawAccount struct {
	Total           string `json:"total"`
	Available       string `json:"available"`
	UnrealisedPnl   string `json:"unrealised_pnl"`
	PositionMargin  string `json:"position_margin"`
	OrderMargin     string `json:"order_margin"`
	Currency        string `json:"currency"`
}

type rawHistoryItem struct {
	Time          int64  `json:"time"`
	Pnl           string `json:"pnl"`
	Side          string `json:"side"`
	Contract      string `json:"contract"`
	Text          string `json:"text"`
	// /position_close 額外欄位（依 Gate v4 swagger）。string 形式回傳。
	// LongPrice 在 side=long 紀錄裡是 entry 均價、ShortPrice 是 exit；side=short 反過來。
	LongPrice     string `json:"long_price"`
	ShortPrice    string `json:"short_price"`
	AccumSize     string `json:"accum_size"`
	MaxSize       string `json:"max_size"`
	FirstOpenTime int64  `json:"first_open_time"`
}

type rawOrderResp struct {
	ID       int64  `json:"id"`
	Contract string `json:"contract"`
	Size     int64  `json:"size"`
	Status   string `json:"status"`
}

type rawContract struct {
	Name             string `json:"name"`
	QuantoMultiplier string `json:"quanto_multiplier"`
}

type rawErr struct {
	Label   string `json:"label"`
	Message string `json:"message"`
	Detail  string `json:"detail"`
}
