package bybit

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

// Positions 取得目前所有 USDT-M 永續倉位。
//
// 過濾規則：size==0 或 side=="" 視為空倉，跳過。
// 注意：UTA 帳戶的 tradeMode 可能恆為 0；M2 不處理 UTA isolated detection，
// 一律以 tradeMode 判斷。
func (c *Client) Positions(ctx context.Context) ([]exchange.Position, error) {
	q := url.Values{}
	q.Set("category", "linear")
	q.Set("settleCoin", "USDT")

	var raw rawPositionList
	if err := c.do(ctx, http.MethodGet, "/position/list", q, nil, &raw); err != nil {
		return nil, err
	}

	out := make([]exchange.Position, 0, len(raw.List))
	for _, it := range raw.List {
		size, _ := exchange.ParseFloat(it.Size)
		if size == 0 || it.Side == "" {
			continue
		}
		out = append(out, exchange.Position{
			Exchange:      c.Name(),
			Symbol:        it.Symbol,
			RawSymbol:     it.Symbol,
			Side:          sideFromBybit(it.Side),
			Size:          size,
			CoinSize:      size, // Bybit V5 USDT-M：size 本身就是 coin 顆數
			EntryPrice:    exchange.MustParseFloat(it.AvgPrice),
			MarkPrice:     exchange.MustParseFloat(it.MarkPrice),
			UnrealizedPnL: exchange.MustParseFloat(it.UnrealisedPnl),
			Leverage:      exchange.MustParseFloat(it.Leverage),
			Notional:      exchange.MustParseFloat(it.PositionValue),
			MarginMode:    marginModeFromBybit(it.TradeMode),
			UpdatedAt:     time.UnixMilli(exchange.MustParseInt(it.UpdatedTime)),
		})
	}
	return out, nil
}

func sideFromBybit(s string) exchange.PositionSide {
	if s == "Sell" {
		return exchange.SideShort
	}
	return exchange.SideLong
}

// marginModeFromBybit 對應 Bybit tradeMode → 統一 "cross" / "isolated"。
// 0 = cross, 1 = isolated (classic)；UTA 帳戶可能恆為 0，視為 cross。
func marginModeFromBybit(mode int) string {
	if mode == 1 {
		return "isolated"
	}
	return "cross"
}
