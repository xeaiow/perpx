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
		// OKX 的 pos 對 SWAP 是「合約張數」；CoinSize 用 notional/mark 推算。
		mark := exchange.MustParseFloat(it.MarkPx)
		notional := exchange.MustParseFloat(it.NotionalUsd)
		coinSize := 0.0
		if mark > 0 {
			coinSize = notional / mark
		}
		out = append(out, exchange.Position{
			Exchange:      "okx",
			Symbol:        normalizeSymbol(it.InstID),
			RawSymbol:     it.InstID,
			Side:          side,
			Size:          math.Abs(pos),
			CoinSize:      coinSize,
			EntryPrice:    exchange.MustParseFloat(it.AvgPx),
			MarkPrice:     mark,
			UnrealizedPnL: exchange.MustParseFloat(it.Upl),
			Leverage:      exchange.MustParseFloat(it.Lever),
			Notional:      notional,
			MarginMode:    it.MgnMode,
			UpdatedAt:     time.UnixMilli(exchange.MustParseInt(it.UTime)),
		})
	}
	return out, nil
}
