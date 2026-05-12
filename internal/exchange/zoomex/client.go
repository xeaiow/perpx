// Package zoomex 是 Bybit V5 的衍生 adapter。
//
// Zoomex 跟 Bybit V5 在簽章與資料結構上是同一套，只差 base URL、path prefix，
// 與一個歷史 endpoint 名（close-pnl vs closed-pnl）。本套件用 composition 包 bybit.Client。
package zoomex

import (
	"context"
	"time"

	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/exchange/bybit"
)

const (
	mainnetBaseURL = "https://openapi.zoomex.com"
	testnetBaseURL = "https://openapi-testnet.zoomex.com"
	pathPrefix     = "/cloud/trade/v3"

	// 歷史 endpoint：Bybit 是 closed-pnl，Zoomex 是 close-pnl（無 d）。
	historyPath = "/position/close-pnl"
)

// Client 是 Zoomex 的 adapter，內部委派給 bybit.Client。
type Client struct {
	inner *bybit.Client
}

// New 建立 Zoomex 客戶端；自動依 testnet 選 baseURL。
func New(c *config.Credentials, rt config.Runtime) *Client {
	baseURL := mainnetBaseURL
	if rt.UseTestnet {
		baseURL = testnetBaseURL
	}
	inner := bybit.NewWithBaseURL(c, rt, baseURL, pathPrefix)
	inner.SetName("zoomex")
	return &Client{inner: inner}
}

// Name 回傳 "zoomex"。
func (c *Client) Name() string { return c.inner.Name() }

// Positions 委派給 bybit.Client.Positions；Exchange 欄位透過 SetName 已被覆寫。
func (c *Client) Positions(ctx context.Context) ([]exchange.Position, error) {
	return c.inner.Positions(ctx)
}

// AvailableBalance 委派給 bybit.Client.AvailableBalance。
func (c *Client) AvailableBalance(ctx context.Context) (float64, error) {
	return c.inner.AvailableBalance(ctx)
}

// History 用 close-pnl 路徑變體（非 closed-pnl）。
func (c *Client) History(ctx context.Context, since time.Time) ([]exchange.ClosedPosition, error) {
	return c.inner.HistoryAtPath(ctx, historyPath, since)
}

// ClosePosition 委派給 bybit.Client.ClosePosition。
func (c *Client) ClosePosition(ctx context.Context, req exchange.CloseRequest) (exchange.CloseResult, error) {
	return c.inner.ClosePosition(ctx, req)
}
