package gate

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) History(ctx context.Context, since time.Time) ([]exchange.ClosedPosition, error) {
	q := url.Values{}
	q.Set("limit", "100")
	if !since.IsZero() {
		// Gate 用秒級 from/to
		q.Set("from", strconv.FormatInt(since.Unix(), 10))
	}

	var arr []rawHistoryItem
	if err := c.do(ctx, http.MethodGet, "/api/v4/futures/usdt/position_close", q, nil, &arr); err != nil {
		return nil, err
	}

	out := make([]exchange.ClosedPosition, 0, len(arr))
	for _, it := range arr {
		side := exchange.SideLong
		if it.Side == "short" {
			side = exchange.SideShort
		}
		out = append(out, exchange.ClosedPosition{
			Exchange:    "gate",
			Symbol:      strings.ReplaceAll(it.Contract, "_", ""),
			Side:        side,
			RealizedPnL: exchange.MustParseFloat(it.Pnl),
			CloseTime:   time.Unix(it.Time, 0),
		})
	}
	return out, nil
}
