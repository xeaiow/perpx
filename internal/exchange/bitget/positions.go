package bitget

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) Positions(ctx context.Context) ([]exchange.Position, error) {
	q := url.Values{}
	q.Set("productType", "USDT-FUTURES")
	q.Set("marginCoin", "USDT")

	var arr []rawPosition
	if err := c.do(ctx, http.MethodGet, "/api/v2/mix/position/all-position", q, nil, &arr); err != nil {
		return nil, err
	}

	out := make([]exchange.Position, 0, len(arr))
	for _, it := range arr {
		total, _ := exchange.ParseFloat(it.Total)
		if total == 0 {
			continue
		}
		side := exchange.SideLong
		if it.HoldSide == "short" {
			side = exchange.SideShort
		}
		mode := "cross"
		if it.MarginMode == "isolated" {
			mode = "isolated"
		}
		mark, _ := exchange.ParseFloat(it.MarkPrice)
		out = append(out, exchange.Position{
			Exchange:      "bitget",
			Symbol:        it.Symbol,
			RawSymbol:     it.Symbol,
			Side:          side,
			Size:          total,
			CoinSize:      total, // Bitget V2 Mix：total 是 coin 顆數
			EntryPrice:    exchange.MustParseFloat(it.OpenPriceAvg),
			MarkPrice:     mark,
			UnrealizedPnL: exchange.MustParseFloat(it.UnrealizedPL),
			Leverage:      exchange.MustParseFloat(it.Leverage),
			Notional:      total * mark,
			MarginMode:    mode,
			UpdatedAt:     time.UnixMilli(exchange.MustParseInt(it.UTime)),
		})
	}
	return out, nil
}
