package bitget

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
	q.Set("productType", "USDT-FUTURES")
	if !since.IsZero() {
		q.Set("startTime", strconv.FormatInt(since.UnixMilli(), 10))
	}

	var raw rawHistoryWrapper
	if err := c.do(ctx, http.MethodGet, "/api/v2/mix/position/history-position", q, nil, &raw); err != nil {
		return nil, err
	}

	out := make([]exchange.ClosedPosition, 0, len(raw.List))
	for _, it := range raw.List {
		side := exchange.SideLong
		if it.HoldSide == "short" {
			side = exchange.SideShort
		}
		out = append(out, exchange.ClosedPosition{
			Exchange:    "bitget",
			Symbol:      it.Symbol,
			Side:        side,
			Size:        exchange.MustParseFloat(it.CloseTotalPos),
			EntryPrice:  exchange.MustParseFloat(it.OpenAvgPrice),
			ExitPrice:   exchange.MustParseFloat(it.CloseAvgPrice),
			RealizedPnL: exchange.MustParseFloat(it.NetProfit),
			OpenTime:    time.UnixMilli(exchange.MustParseInt(it.CTime)),
			CloseTime:   time.UnixMilli(exchange.MustParseInt(it.UTime)),
		})
	}
	return out, nil
}
