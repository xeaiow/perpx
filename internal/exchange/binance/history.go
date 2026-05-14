package binance

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

// History 從 /fapi/v1/income 聚合。
//
// Binance 沒有原生 close-position 端點；income 流水是按 trade fill 一筆一列，
// 多 fill 平倉會回多筆 REALIZED_PNL。為了讓 UI「一倉一列」的視覺成立，這裡
// 按 (symbol, 一分鐘 bucket) 聚合 PnL；同一分鐘對同一 symbol 的多筆 fill 視
// 為「一次平倉動作」。Side / Size / EntryPrice / ExitPrice 仍無法從 income
// 流水推得，留 zero（UI 顯示 —）。
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

	type key struct {
		symbol string
		bucket int64 // unix minute
	}
	type agg struct {
		pnl     float64
		latest  int64 // ms — 取 bucket 內最後一筆的 time 顯示
	}
	groups := make(map[key]*agg)

	for _, it := range arr {
		if it.IncomeType != "REALIZED_PNL" {
			continue
		}
		if it.Asset != "USDT" {
			continue
		}
		k := key{symbol: it.Symbol, bucket: it.Time / 60000}
		a, ok := groups[k]
		if !ok {
			a = &agg{}
			groups[k] = a
		}
		a.pnl += exchange.MustParseFloat(it.Income)
		if it.Time > a.latest {
			a.latest = it.Time
		}
	}

	out := make([]exchange.ClosedPosition, 0, len(groups))
	for k, a := range groups {
		out = append(out, exchange.ClosedPosition{
			Exchange:    "binance",
			Symbol:      k.symbol,
			RealizedPnL: a.pnl,
			CloseTime:   time.UnixMilli(a.latest),
		})
	}
	return out, nil
}
