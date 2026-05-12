package gate

import (
	"context"
	"net/http"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) AvailableBalance(ctx context.Context) (float64, error) {
	var acc rawAccount
	if err := c.do(ctx, http.MethodGet, "/api/v4/futures/usdt/accounts", nil, nil, &acc); err != nil {
		return 0, err
	}
	v, _ := exchange.ParseFloat(acc.Available)
	return v, nil
}
