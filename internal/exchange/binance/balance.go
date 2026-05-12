package binance

import (
	"context"
	"net/http"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) AvailableBalance(ctx context.Context) (float64, error) {
	var arr []rawBalanceEntry
	if err := c.do(ctx, http.MethodGet, "/fapi/v3/balance", nil, true, &arr); err != nil {
		return 0, err
	}
	for _, b := range arr {
		if b.Asset == "USDT" {
			v, _ := exchange.ParseFloat(b.AvailableBalance)
			return v, nil
		}
	}
	return 0, nil
}
