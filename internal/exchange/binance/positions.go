package binance

import (
	"context"
	"math"
	"net/http"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) Positions(ctx context.Context) ([]exchange.Position, error) {
	var arr []rawPosition
	// v2 over v3: v3 omits `leverage` and `marginType` from the response (they
	// became account-level in v3). v2 still returns both, and that's the only
	// reason we'd want to call this endpoint — the extra v3 fields (askNotional,
	// adl, maintMargin, …) we don't use.
	if err := c.do(ctx, http.MethodGet, "/fapi/v2/positionRisk", nil, true, &arr); err != nil {
		return nil, err
	}

	out := make([]exchange.Position, 0, len(arr))
	for _, it := range arr {
		// 只要 USDT 結算
		if it.MarginAsset != "" && it.MarginAsset != "USDT" {
			continue
		}
		amt, _ := exchange.ParseFloat(it.PositionAmt)
		if amt == 0 {
			continue
		}
		side := exchange.SideLong
		if amt < 0 {
			side = exchange.SideShort
		}
		// Hedge mode：positionSide=LONG/SHORT 也可能正/負；以 amt 正負為準。
		out = append(out, exchange.Position{
			Exchange:      "binance",
			Symbol:        it.Symbol,
			RawSymbol:     it.Symbol,
			Side:          side,
			Size:          math.Abs(amt),
			CoinSize:      math.Abs(amt), // Binance USDⓈ-M：positionAmt 是 coin 顆數
			EntryPrice:    exchange.MustParseFloat(it.EntryPrice),
			MarkPrice:     exchange.MustParseFloat(it.MarkPrice),
			UnrealizedPnL: exchange.MustParseFloat(it.UnRealizedProfit),
			Leverage:      exchange.MustParseFloat(it.Leverage),
			Notional:      math.Abs(exchange.MustParseFloat(it.Notional)),
			MarginMode:    it.MarginType,
			UpdatedAt:     time.UnixMilli(it.UpdateTime),
		})
	}
	return out, nil
}
