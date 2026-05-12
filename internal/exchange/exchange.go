// Package exchange 定義所有交易所 adapter 共用的抽象。
//
// 每個交易所一個子套件實作 Exchange interface，registry 依 config 注入。
package exchange

import (
	"context"
	"time"
)

// PositionSide 統一多空方向。各家 API 用詞不同（"long"/"short"、"Buy"/"Sell"、
// 數量正負號），adapter 內部標準化成這個型別。
type PositionSide string

const (
	SideLong  PositionSide = "long"
	SideShort PositionSide = "short"
)

// Position 是當前未平倉的永續合約倉位（USDT-M）。
type Position struct {
	Exchange      string       // adapter 識別："binance" 等
	Symbol        string       // 標準化 symbol，例如 "BTCUSDT"。跨所比對用
	RawSymbol     string       // 各交易所原始 symbol，平倉時用這個（OKX 是 "BTC-USDT-SWAP" 等）
	Side          PositionSide
	RawSide       string       // 各交易所原樣的 side 字串（OKX: "long" / "short" / "net"；其他家可留空）；close 用
	Size          float64      // 交易所原樣回的數量；多數家是 coin 顆數，Gate 是 contracts
	CoinSize      float64      // 統一單位的 coin 顆數；UI 用這個顯示「實際幣量」
	EntryPrice    float64
	MarkPrice     float64
	UnrealizedPnL float64      // USDT
	Leverage      float64
	Notional      float64      // size * markPrice，方便排序
	MarginMode    string       // "cross" / "isolated"
	UpdatedAt     time.Time
}

// ClosedPosition 是歷史已平倉倉位。
//
// 注意：欄位語意各家不同。Binance 沒有原生端點，由 income 流水聚合。
// RealizedPnL 是否含資金費由各 adapter 文件註明。
type ClosedPosition struct {
	Exchange    string
	Symbol      string
	Side        PositionSide
	Size        float64
	EntryPrice  float64
	ExitPrice   float64
	RealizedPnL float64 // USDT
	OpenTime    time.Time
	CloseTime   time.Time
}

// CloseRequest 平倉請求。對應 UI 「按 x 確認 y」流程。
type CloseRequest struct {
	Symbol     string       // 用 Position.RawSymbol
	Side       PositionSide // 要平的方向。Hedge mode 必填
	RawSide    string       // 各交易所原樣的 side 字串；adapter 內部優先使用
	Size       float64      // 平多少；0 表示全部
	MarginMode string       // adapter 可能用到（OKX 平倉要這個）
}

// CloseResult 平倉結果。OrderID 可能為空（OKX close-position 不回 OrderID）。
type CloseResult struct {
	OrderID   string
	Symbol    string
	Side      PositionSide
	Size      float64
	Timestamp time.Time
}

// Exchange 是每個交易所 adapter 要實作的介面。
//
// 所有方法都接受 context.Context；timeout/取消由呼叫端控制。
// 範圍限定 USDT-M 永續合約；其他 settle coin 不在 scope 內。
type Exchange interface {
	Name() string
	Positions(ctx context.Context) ([]Position, error)
	AvailableBalance(ctx context.Context) (float64, error)
	History(ctx context.Context, since time.Time) ([]ClosedPosition, error)
	ClosePosition(ctx context.Context, req CloseRequest) (CloseResult, error)
}
