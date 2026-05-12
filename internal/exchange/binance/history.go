package binance

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

// History 從 /fapi/v1/income 聚合。每筆 REALIZED_PNL → 一個 ClosedPosition。
// Binance 沒有原生 close-position 端點；簡化實作只填 Symbol/RealizedPnL/CloseTime，
// Side、Size、EntryPrice、ExitPrice、OpenTime 留 zero。UI 容忍空白。
func (c *Client) History(ctx context.Context, since time.Time) ([]exchange.ClosedPosition, error) {
	q := url.Values{}
	q.Set("incomeType", "REALIZED_PNL")
	q.Set("limit", "1000")
	if !since.IsZero() {
		q.Set("startTime", strconv.FormatInt(since.UnixMilli(), 10))
	}

	var arr []rawIncome
	if err := c.do(ctx, http.MethodGet, "/fapi/v1/income", q, true, &arr); err != nil {
		return nil, err
	}

	out := make([]exchange.ClosedPosition, 0, len(arr))
	for _, it := range arr {
		if it.IncomeType != "REALIZED_PNL" {
			continue
		}
		if it.Asset != "USDT" {
			continue
		}
		out = append(out, exchange.ClosedPosition{
			Exchange:    "binance",
			Symbol:      it.Symbol,
			RealizedPnL: exchange.MustParseFloat(it.Income),
			CloseTime:   time.UnixMilli(it.Time),
		})
	}
	return out, nil
}
