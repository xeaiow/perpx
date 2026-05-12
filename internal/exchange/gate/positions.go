package gate

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

func (c *Client) Positions(ctx context.Context) ([]exchange.Position, error) {
	var arr []rawPosition
	if err := c.do(ctx, http.MethodGet, "/api/v4/futures/usdt/positions", nil, nil, &arr); err != nil {
		return nil, err
	}

	out := make([]exchange.Position, 0, len(arr))
	for _, it := range arr {
		if it.Size == 0 {
			continue
		}
		side := exchange.SideLong
		if it.Size < 0 {
			side = exchange.SideShort
		}
		// Size 保持交易所原樣（contracts）；CoinSize 由 value/markPrice 推算，
		// 避免額外打一支 /contracts/{contract} 拿 quanto_multiplier。
		mark := exchange.MustParseFloat(it.MarkPrice)
		notional := exchange.MustParseFloat(it.Value)
		coinSize := 0.0
		if mark > 0 {
			coinSize = notional / mark
		}
		// leverage：跨倉時 raw "0"，實際倍率在 cross_leverage_limit。
		lev := exchange.MustParseFloat(it.Leverage)
		if lev == 0 {
			lev = exchange.MustParseFloat(it.CrossLeverageLimit)
		}
		sym := strings.ReplaceAll(it.Contract, "_", "")
		out = append(out, exchange.Position{
			Exchange:      "gate",
			Symbol:        sym,
			RawSymbol:     it.Contract,
			Side:          side,
			Size:          math.Abs(it.Size),
			CoinSize:      coinSize,
			EntryPrice:    exchange.MustParseFloat(it.EntryPrice),
			MarkPrice:     mark,
			UnrealizedPnL: exchange.MustParseFloat(it.UnrealisedPnl),
			Leverage:      lev,
			Notional:      notional,
			MarginMode:    "cross",
			UpdatedAt:     time.Now(),
		})
	}
	return out, nil
}
