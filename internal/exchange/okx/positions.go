package okx

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) Positions(ctx context.Context) ([]exchange.Position, error) {
	q := url.Values{}
	q.Set("instType", "SWAP")

	var arr []rawPosition
	if err := c.do(ctx, http.MethodGet, "/api/v5/account/positions", q, nil, &arr); err != nil {
		return nil, err
	}

	out := make([]exchange.Position, 0, len(arr))
	for _, it := range arr {
		// 只要 USDT 結算
		if it.Ccy != "" && it.Ccy != "USDT" {
			continue
		}
		pos, _ := exchange.ParseFloat(it.Pos)
		if pos == 0 {
			continue
		}
		side := exchange.SideLong
		switch it.PosSide {
		case "long":
			side = exchange.SideLong
		case "short":
			side = exchange.SideShort
		case "net", "":
			if pos < 0 {
				side = exchange.SideShort
			}
		}
		out = append(out, exchange.Position{
			Exchange:      "okx",
			Symbol:        normalizeSymbol(it.InstID),
			RawSymbol:     it.InstID,
			Side:          side,
			Size:          math.Abs(pos),
			EntryPrice:    exchange.MustParseFloat(it.AvgPx),
			MarkPrice:     exchange.MustParseFloat(it.MarkPx),
			UnrealizedPnL: exchange.MustParseFloat(it.Upl),
			Leverage:      exchange.MustParseFloat(it.Lever),
			Notional:      exchange.MustParseFloat(it.NotionalUsd),
			MarginMode:    it.MgnMode,
			UpdatedAt:     time.UnixMilli(exchange.MustParseInt(it.UTime)),
		})
	}
	return out, nil
}
