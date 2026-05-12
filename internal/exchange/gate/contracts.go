package gate

import (
	"context"
	"net/http"

	"github.com/yourname/poscli/internal/exchange"
)

// getMultiplier 取得指定 contract 的 quanto_multiplier，並 cache。
// 失敗（找不到 contract 或 API 錯）時回 1 並把錯誤吞掉 — 這只是 size 換算，
// 退而求其次顯示 contracts 不致命。
func (c *Client) getMultiplier(ctx context.Context, contract string) float64 {
	c.mu.RLock()
	v, ok := c.multipliers[contract]
	c.mu.RUnlock()
	if ok {
		return v
	}
	var ct rawContract
	if err := c.do(ctx, http.MethodGet, "/api/v4/futures/usdt/contracts/"+contract, nil, nil, &ct); err != nil {
		return 1
	}
	m, _ := exchange.ParseFloat(ct.QuantoMultiplier)
	if m == 0 {
		m = 1
	}
	c.mu.Lock()
	c.multipliers[contract] = m
	c.mu.Unlock()
	return m
}
