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
	q.Set("limit", "100")
	if !since.IsZero() {
		// OKX `after`/`before` 是「分頁游標」（取早於/晚於這個值的紀錄），
		// 不是時間範圍。要做時間範圍要用 `begin` / `end`（皆為 ms timestamp）。
		// 之前用 after 等於送了「早於 since」、把 since 之後的全濾掉，這是 bug。
		q.Set("begin", strconv.FormatInt(since.UnixMilli(), 10))
		q.Set("end", strconv.FormatInt(time.Now().UnixMilli(), 10))
	}

	var arr []rawClosedPosition
	if err := c.do(ctx, http.MethodGet, "/api/v5/account/positions-history", q, nil, &arr); err != nil {
		return nil, err
	}

	out := make([]exchange.ClosedPosition, 0, len(arr))
	for _, it := range arr {
		closeMs := exchange.MustParseInt(it.UTime)
		// OKX 對 begin/end 不是強制 honor、保險起見再做一次 since 過濾。
		if !since.IsZero() && closeMs < since.UnixMilli() {
			continue
		}
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
