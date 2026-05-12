package okx

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) History(ctx context.Context, since time.Time) ([]exchange.ClosedPosition, error) {
	q := url.Values{}
	q.Set("instType", "SWAP")
	if !since.IsZero() {
		// OKX 用 after/before 為 ID 或 ms timestamp；用 after = since-1 取 since 之後
		q.Set("after", strconv.FormatInt(since.UnixMilli(), 10))
	}

	var arr []rawClosedPosition
	if err := c.do(ctx, http.MethodGet, "/api/v5/account/positions-history", q, nil, &arr); err != nil {
		return nil, err
	}

	out := make([]exchange.ClosedPosition, 0, len(arr))
	for _, it := range arr {
		side := exchange.SideLong
		if it.PosSide == "short" {
			side = exchange.SideShort
		}
		pnl := it.RealizedPnl
		if pnl == "" {
			pnl = it.Pnl
		}
		out = append(out, exchange.ClosedPosition{
			Exchange:    "okx",
			Symbol:      normalizeSymbol(it.InstID),
			Side:        side,
			Size:        exchange.MustParseFloat(it.CloseTotal),
			EntryPrice:  exchange.MustParseFloat(it.OpenAvgPx),
			ExitPrice:   exchange.MustParseFloat(it.CloseAvgPx),
			RealizedPnL: exchange.MustParseFloat(pnl),
			OpenTime:    time.UnixMilli(exchange.MustParseInt(it.OpenTime)),
			CloseTime:   time.UnixMilli(exchange.MustParseInt(it.UTime)),
		})
	}
	return out, nil
}
