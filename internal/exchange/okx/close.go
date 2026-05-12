package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) ClosePosition(ctx context.Context, req exchange.CloseRequest) (exchange.CloseResult, error) {
	mgnMode := req.MarginMode
	if mgnMode == "" {
		mgnMode = "cross"
	}
	body := map[string]any{
		"instId":  req.Symbol,
		"mgnMode": mgnMode,
		"ccy":     "USDT",
	}
	// posSide handling depends on the account's position mode (read from raw
	// `posSide` field captured at fetch time):
	//   - "net"            → omit posSide (passing it triggers code=51000)
	//   - "long" / "short" → echo back verbatim (long_short_mode requirement)
	//   - empty            → fall back to normalized Side mapping; better than
	//                         no value, though it may still 51000 on net-mode
	//                         accounts whose fetch happened before this fix
	switch req.RawSide {
	case "long", "short":
		body["posSide"] = req.RawSide
	case "net":
		// omit
	default:
		switch req.Side {
		case exchange.SideLong:
			body["posSide"] = "long"
		case exchange.SideShort:
			body["posSide"] = "short"
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return exchange.CloseResult{}, fmt.Errorf("okx close marshal: %w", err)
	}

	var arr []rawCloseResult
	if err := c.do(ctx, http.MethodPost, "/api/v5/trade/close-position", nil, raw, &arr); err != nil {
		return exchange.CloseResult{}, err
	}
	// OKX 不回 orderID
	return exchange.CloseResult{
		OrderID:   "",
		Symbol:    req.Symbol,
		Side:      req.Side,
		Size:      req.Size,
		Timestamp: c.now(),
	}, nil
}
