package okx

import (
	"context"
	"net/http"
	"net/url"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) AvailableBalance(ctx context.Context) (float64, error) {
	q := url.Values{}
	q.Set("ccy", "USDT")

	var arr []rawBalanceData
	if err := c.do(ctx, http.MethodGet, "/api/v5/account/balance", q, nil, &arr); err != nil {
		return 0, err
	}
	for _, b := range arr {
		for _, d := range b.Details {
			if d.Ccy != "USDT" {
				continue
			}
			// 優先 availEq（含 unrealized PnL 的可用）；若空退 availBal。
			if v, _ := exchange.ParseFloat(d.AvailEq); v > 0 {
				return v, nil
			}
			v, _ := exchange.ParseFloat(d.AvailBal)
			return v, nil
		}
	}
	return 0, nil
}
