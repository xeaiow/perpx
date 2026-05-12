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
		mult := c.getMultiplier(ctx, it.Contract)
		coinSize := math.Abs(it.Size) * mult
		sym := strings.ReplaceAll(it.Contract, "_", "")
		out = append(out, exchange.Position{
			Exchange:      "gate",
			Symbol:        sym,
			RawSymbol:     it.Contract,
			Side:          side,
			Size:          coinSize,
			EntryPrice:    exchange.MustParseFloat(it.EntryPrice),
			MarkPrice:     exchange.MustParseFloat(it.MarkPrice),
			UnrealizedPnL: exchange.MustParseFloat(it.UnrealisedPnl),
			Leverage:      exchange.MustParseFloat(it.Leverage),
			Notional:      exchange.MustParseFloat(it.Value),
			MarginMode:    "cross",
			UpdatedAt:     time.Now(),
		})
	}
	return out, nil
}
