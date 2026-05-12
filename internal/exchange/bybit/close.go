package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/yourname/poscli/internal/exchange"
)

// ClosePosition 透過 reduce-only market order 平倉。
//
// M2 限制：僅支援 one-way（positionIdx=0）。Hedge mode 視為錯誤。
func (c *Client) ClosePosition(ctx context.Context, req exchange.CloseRequest) (exchange.CloseResult, error) {
	side := "Sell"
	if req.Side == exchange.SideShort {
		side = "Buy"
	}

	body := map[string]any{
		"category":    "linear",
		"symbol":      req.Symbol,
		"side":        side,
		"orderType":   "Market",
		"reduceOnly":  true,
		"positionIdx": 0,
	}
	if req.Size > 0 {
		// 數量字串保留至小數點 8 位，去尾零。Bybit 接受字串型 qty。
		body["qty"] = strconv.FormatFloat(req.Size, 'f', -1, 64)
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return exchange.CloseResult{}, fmt.Errorf("bybit close marshal: %w", err)
	}

	var result rawOrderCreateResult
	if err := c.do(ctx, http.MethodPost, "/order/create", nil, raw, &result); err != nil {
		return exchange.CloseResult{}, err
	}

	return exchange.CloseResult{
		OrderID:   result.OrderID,
		Symbol:    req.Symbol,
		Side:      req.Side,
		Size:      req.Size,
		Timestamp: c.now(),
	}, nil
}
