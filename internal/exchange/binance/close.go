package binance

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/yourname/poscli/internal/exchange"
)

// ClosePosition 用 reduce-only market order 平倉（one-way 模式）。
// Hedge mode 支援延後：M4 階段以 reduceOnly + positionSide=BOTH 假設 one-way。
func (c *Client) ClosePosition(ctx context.Context, req exchange.CloseRequest) (exchange.CloseResult, error) {
	side := "SELL"
	if req.Side == exchange.SideShort {
		side = "BUY"
	}
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", side)
	params.Set("type", "MARKET")
	if req.Size > 0 {
		params.Set("quantity", strconv.FormatFloat(req.Size, 'f', -1, 64))
	}
	params.Set("reduceOnly", "true")
	params.Set("positionSide", "BOTH")

	var resp rawOrderResp
	if err := c.do(ctx, http.MethodPost, "/fapi/v1/order", params, true, &resp); err != nil {
		return exchange.CloseResult{}, err
	}
	return exchange.CloseResult{
		OrderID:   strconv.FormatInt(resp.OrderID, 10),
		Symbol:    req.Symbol,
		Side:      req.Side,
		Size:      req.Size,
		Timestamp: c.now(),
	}, nil
}
