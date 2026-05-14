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
		// side=long  →  入場價 = long_price (買入)、出場價 = short_price (賣出)
		// side=short →  入場價 = short_price (賣空)、出場價 = long_price (買回)
		entry := exchange.MustParseFloat(it.LongPrice)
		exit := exchange.MustParseFloat(it.ShortPrice)
		if side == exchange.SideShort {
			entry, exit = exit, entry
		}
		var openT time.Time
		if it.FirstOpenTime > 0 {
			openT = time.Unix(it.FirstOpenTime, 0)
		}
		out = append(out, exchange.ClosedPosition{
			Exchange:    "gate",
			Symbol:      strings.ReplaceAll(it.Contract, "_", ""),
			Side:        side,
			Size:        exchange.MustParseFloat(it.AccumSize),
			EntryPrice:  entry,
			ExitPrice:   exit,
			RealizedPnL: exchange.MustParseFloat(it.Pnl),
			OpenTime:    openT,
			CloseTime:   time.Unix(it.Time, 0),
		})
	}
	return out, nil
}
