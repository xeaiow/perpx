package exchange

import "errors"

// 共用 sentinel errors。每個 adapter 把家內錯誤碼對應成這些通用值，
// 上層用 errors.Is 判斷後決定如何呈現給使用者。
var (
	// ErrAuth 表示鑑權失敗（API key 錯、簽章錯、權限不足）。
	ErrAuth = errors.New("exchange: auth failed")

	// ErrRateLimit 表示被速率限制（429、retCode=10006、code=50011 等）。
	ErrRateLimit = errors.New("exchange: rate limited")

	// ErrServerSide 表示交易所端錯誤（5xx）。
	ErrServerSide = errors.New("exchange: server error")

	// ErrClientSide 表示本地請求有誤但非鑑權問題（其他 4xx、parameter error）。
	ErrClientSide = errors.New("exchange: client error")

	// ErrParseResp 表示 JSON 結構或型別跟預期不符。
	ErrParseResp = errors.New("exchange: malformed response")
)
