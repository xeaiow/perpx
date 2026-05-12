package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/yourname/poscli/internal/exchange"
)

// ClosePosition 用 Gate 的「size=0 + close=true」idiom 平倉。
// M4 假設 one-way（mode=single）；hedge mode 未實作。
func (c *Client) ClosePosition(ctx context.Context, req exchange.CloseRequest) (exchange.CloseResult, error) {
	body := map[string]any{
		"contract":    req.Symbol,
		"size":        0,
		"price":       "0",
		"tif":         "ioc",
		"close":       true,
		"reduce_only": true,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return exchange.CloseResult{}, fmt.Errorf("gate close marshal: %w", err)
	}
	var resp rawOrderResp
	if err := c.do(ctx, http.MethodPost, "/api/v4/futures/usdt/orders", nil, raw, &resp); err != nil {
		return exchange.CloseResult{}, err
	}
	return exchange.CloseResult{
		OrderID:   strconv.FormatInt(resp.ID, 10),
		Symbol:    req.Symbol,
		Side:      req.Side,
		Size:      req.Size,
		Timestamp: c.now(),
	}, nil
}
