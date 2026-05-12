package bybit

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

// History 取得已平倉的盈虧紀錄。
//
// since 為 zero 時不送 startTime，由 Bybit 回最近預設區間（約 7 日）。
// 注意：raw.Side 是「平倉那筆 fill」的 side；倉位本身方向相反。
func (c *Client) History(ctx context.Context, since time.Time) ([]exchange.ClosedPosition, error) {
	return c.HistoryAtPath(ctx, "/position/closed-pnl", since)
}

// HistoryAtPath 跟 History 一樣，但允許指定 path（Zoomex 用 close-pnl）。
func (c *Client) HistoryAtPath(ctx context.Context, path string, since time.Time) ([]exchange.ClosedPosition, error) {
	q := url.Values{}
	q.Set("category", "linear")
	q.Set("limit", "200")
	if !since.IsZero() {
		q.Set("startTime", strconv.FormatInt(since.UnixMilli(), 10))
	}

	var raw rawClosedPnlList
	if err := c.do(ctx, http.MethodGet, path, q, nil, &raw); err != nil {
		return nil, err
	}

	out := make([]exchange.ClosedPosition, 0, len(raw.List))
	for _, it := range raw.List {
		// 平倉那筆 side 是 Buy 表示原本是空單；Sell 則原本是多單。
		var posSide exchange.PositionSide = exchange.SideShort
		if it.Side == "Sell" {
			posSide = exchange.SideLong
		}
		out = append(out, exchange.ClosedPosition{
			Exchange:    c.Name(),
			Symbol:      it.Symbol,
			Side:        posSide,
			Size:        exchange.MustParseFloat(it.ClosedSize),
			EntryPrice:  exchange.MustParseFloat(it.AvgEntryPrice),
			ExitPrice:   exchange.MustParseFloat(it.AvgExitPrice),
			RealizedPnL: exchange.MustParseFloat(it.ClosedPnl),
			OpenTime:    time.UnixMilli(exchange.MustParseInt(it.CreatedTime)),
			CloseTime:   time.UnixMilli(exchange.MustParseInt(it.UpdatedTime)),
		})
	}
	return out, nil
}
