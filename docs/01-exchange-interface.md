# 01 — Exchange interface contract

Every exchange adapter implements `exchange.Exchange`. Read this before
implementing or modifying any adapter.

## The interface

```go
// internal/exchange/exchange.go
type Exchange interface {
    Name() string
    Positions(ctx context.Context) ([]Position, error)
    AvailableBalance(ctx context.Context) (float64, error)
    History(ctx context.Context, since time.Time) ([]ClosedPosition, error)
    ClosePosition(ctx context.Context, req CloseRequest) (CloseResult, error)
}
```

All methods take `context.Context`; honor cancellation.

## Data types

```go
type PositionSide string
const (
    SideLong  PositionSide = "long"
    SideShort PositionSide = "short"
)

type Position struct {
    Exchange      string       // adapter identifier ("binance", ...)
    Symbol        string       // normalized: "BTCUSDT" (concat base+quote)
    RawSymbol     string       // original from API; pass back when closing
    Side          PositionSide
    Size          float64      // absolute value (positive)
    EntryPrice    float64
    MarkPrice     float64
    UnrealizedPnL float64      // USDT
    Leverage      float64
    Notional      float64      // size * markPrice; used for Accounts tab equity sum
    MarginMode    string       // "cross" or "isolated"
    UpdatedAt     time.Time
}

type ClosedPosition struct {
    Exchange    string
    Symbol      string
    Side        PositionSide
    Size        float64
    EntryPrice  float64
    ExitPrice   float64
    RealizedPnL float64
    OpenTime    time.Time
    CloseTime   time.Time
}

type CloseRequest struct {
    Symbol     string       // pass Position.RawSymbol
    Side       PositionSide // required in hedge mode
    Size       float64      // 0 = close all
    MarginMode string       // "cross" or "isolated"; needed by OKX
}

type CloseResult struct {
    OrderID   string       // may be empty (OKX close-position doesn't return one)
    Symbol    string
    Side      PositionSide
    Size      float64
    Timestamp time.Time
}
```

## Normalization rules

These apply to all adapters. **Do not** leak exchange-specific shapes upward.

### Symbol

- **Normalized form**: `BASEQUOTE`, uppercase, no separator. Examples:
  `BTCUSDT`, `ETHUSDT`, `SOLUSDT`.
- **Source mappings**:

  | Exchange | Native shape | Normalize |
  |---|---|---|
  | Binance | `BTCUSDT` | unchanged |
  | OKX | `BTC-USDT-SWAP` | drop `-SWAP`, drop `-`; `BTCUSDT` |
  | Bybit | `BTCUSDT` | unchanged |
  | Bitget | `BTCUSDT` | unchanged |
  | Gate | `BTC_USDT` | drop `_`; `BTCUSDT` |
  | Zoomex | `BTCUSDT` | unchanged (same as Bybit) |

- **`RawSymbol`** holds the native shape. `ClosePosition` uses this verbatim.

### Side

Normalize **always** to `SideLong` or `SideShort`. Source signals:

| Exchange | Long signal | Short signal |
|---|---|---|
| Binance | `positionAmt > 0` | `positionAmt < 0` |
| Binance hedge mode | `positionSide=LONG` | `positionSide=SHORT` |
| OKX | `posSide=long` or `pos > 0` | `posSide=short` or `pos < 0` |
| Bybit | `side=Buy` | `side=Sell` |
| Bitget | `holdSide=long` | `holdSide=short` |
| Gate | `size > 0` | `size < 0` |
| Zoomex | `side=Buy` | `side=Sell` |

`Size` is **always absolute value** in our type. Direction lives in `Side`.

### Numeric values

Almost every exchange returns numbers as JSON strings. Use a helper:

```go
// parseFloat tolerates "" and returns 0; non-empty must parse cleanly.
func parseFloat(s string) (float64, error) {
    if s == "" { return 0, nil }
    return strconv.ParseFloat(s, 64)
}
```

Put this in `internal/exchange/types.go` so all adapters share it.

### Time

API responses are millisecond Unix timestamps in JSON strings (most) or
numbers (some). Convert to `time.Time` via `time.UnixMilli(n)`.

### Filtering empty positions

Some APIs return all symbols including ones with `size=0`. Drop those:

```go
if p.Size == 0 { continue }
```

Do this in the adapter, not in UI.

## Empty position semantics

**Empty position**: an entry in the API response where `size == 0`. Skip these.

**No positions at all**: return `[]Position{}` (empty slice), not `nil`. UI
expects to iterate; nil is fine in Go but explicit empty is clearer.

## Error handling

```go
// internal/exchange/errors.go
var (
    ErrAuth        = errors.New("exchange: auth failed")        // 401, signature bad
    ErrRateLimit   = errors.New("exchange: rate limited")        // 429
    ErrServerSide  = errors.New("exchange: server error")        // 5xx
    ErrClientSide  = errors.New("exchange: client error")        // other 4xx
    ErrParseResp   = errors.New("exchange: malformed response")  // unexpected JSON
)
```

Wrap with context, e.g.:

```go
return nil, fmt.Errorf("bybit positions: %w: %s", ErrRateLimit, retMsg)
```

UI can then `errors.Is(err, exchange.ErrAuth)` to format an actionable message.

## Time sync

Several exchanges reject requests where `timestamp` differs from server time
by more than a window (Binance `recvWindow`, OKX 30s, Bybit `recvWindow`, etc.).

Each adapter's client should:

1. On first call (lazy), fetch server time via the exchange's public time endpoint.
2. Compute `delta = serverTime - localTime`.
3. Apply `delta` to all subsequent signed-request timestamps.
4. Re-sync if a request fails with the recv-window error code (usually one of
   `-1021` Binance, `50102` OKX, `10002` Bybit).

Put this in `client.go` of each adapter so it's shared across all methods.

## Test contract

Every adapter must have:

1. **Signature unit test** with a known fixture. Each exchange's docs include
   a worked example showing a payload and the expected signature. Use that.

2. **HTTP roundtrip tests** with `httptest.Server`:
   - Each method's happy path (positions, balance, history, close)
   - Each method's error mapping (auth fail, rate limit, malformed JSON)
   - Normalization correctness (symbol, side, size sign)
   - Empty-positions filtering

3. **Optional integration test**, gated by env var:
   ```go
   if os.Getenv("INTEGRATION_BYBIT") == "" {
       t.Skip("set INTEGRATION_BYBIT=1 to run with testnet credentials")
   }
   ```
   Uses real testnet credentials from env vars. Not run by `go test ./...` default.

## Registry

`internal/exchange/registry.go` (to be created in M2) builds a
`map[ExchangeName]Exchange` from a `*config.LoadResult`. Adapters register
their constructor via init or explicit factory call.

```go
// Sketch — flesh out in M2
func NewRegistry(r *config.LoadResult) (map[config.ExchangeName]Exchange, error) {
    out := make(map[config.ExchangeName]Exchange)
    for name, creds := range r.Credentials {
        ex, err := newAdapter(name, creds, r.Config.Runtime)
        if err != nil {
            return nil, fmt.Errorf("init %s adapter: %w", name, err)
        }
        out[name] = ex
    }
    return out, nil
}

func newAdapter(name config.ExchangeName, c *config.Credentials, rt config.Runtime) (Exchange, error) {
    switch name {
    case config.Bybit:  return bybit.New(c, rt), nil
    case config.Zoomex: return zoomex.New(c, rt), nil
    // ...
    default:
        return nil, fmt.Errorf("unknown exchange: %s", name)
    }
}
```

This keeps UI code free of `if name == "binance"` switches.
