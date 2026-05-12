package bitget

import (
	"context"
	"net/http"
	"net/url"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) AvailableBalance(ctx context.Context) (float64, error) {
	q := url.Values{}
	q.Set("productType", "USDT-FUTURES")

	var arr []rawAccount
	if err := c.do(ctx, http.MethodGet, "/api/v2/mix/account/accounts", q, nil, &arr); err != nil {
		return 0, err
	}
	for _, a := range arr {
		if a.MarginCoin == "USDT" {
			v, _ := exchange.ParseFloat(a.Available)
			return v, nil
		}
	}
	return 0, nil
}
