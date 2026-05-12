package bitget

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) ClosePosition(ctx context.Context, req exchange.CloseRequest) (exchange.CloseResult, error) {
	body := map[string]any{
		"symbol":      req.Symbol,
		"productType": "USDT-FUTURES",
	}
	switch req.Side {
	case exchange.SideLong:
		body["holdSide"] = "long"
	case exchange.SideShort:
		body["holdSide"] = "short"
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return exchange.CloseResult{}, fmt.Errorf("bitget close marshal: %w", err)
	}

	var result rawCloseResult
	if err := c.do(ctx, http.MethodPost, "/api/v2/mix/order/close-positions", nil, raw, &result); err != nil {
		return exchange.CloseResult{}, err
	}
	if len(result.FailureList) > 0 {
		f := result.FailureList[0]
		return exchange.CloseResult{}, fmt.Errorf("bitget close failed: %s (%s)", f.ErrMsg, f.ErrCode)
	}
	orderID := ""
	if len(result.SuccessList) > 0 {
		orderID = result.SuccessList[0].OrderID
	}
	return exchange.CloseResult{
		OrderID:   orderID,
		Symbol:    req.Symbol,
		Side:      req.Side,
		Size:      req.Size,
		Timestamp: c.now(),
	}, nil
}
