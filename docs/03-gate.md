# 03 — Gate.io V4 Futures (USDT settled) adapter

## Endpoints

| Function | Method | Path | Notes |
|---|---|---|---|
| Server time | `GET` | `/api/v4/spot/time` | Public |
| Positions | `GET` | `/api/v4/futures/usdt/positions` | All positions in USDT-settled market |
| Account | `GET` | `/api/v4/futures/usdt/accounts` | USDT-M futures wallet |
| History closes | `GET` | `/api/v4/futures/usdt/position_close` | Native |
| Place order | `POST` | `/api/v4/futures/usdt/orders` | Close = size 0 + close=true |

**Base URL**: `https://api.gateio.ws`

For futures specifically there's `https://fx-api.gateio.ws` but the unified
`api.gateio.ws` also works. Use `api.gateio.ws` for consistency.

Testnet: `https://fx-api-testnet.gateio.ws`.

## Authentication

Headers:

| Header | Value |
|---|---|
| `KEY` | API key |
| `Timestamp` | Unix seconds (NOT milliseconds — yes, really) |
| `SIGN` | hex of HMAC-SHA512 (NOT SHA-256 — yes, really) |
| `Content-Type` | `application/json` for POST |

**Two things make Gate stand out**:
1. Timestamp is **Unix seconds**, not milliseconds.
2. Hash is **HMAC-SHA512**, not SHA-256.

Tests must guard against accidentally porting the SHA-256 logic from other adapters.

**Signing**:

```
hashedBody = hex(SHA512(body))               // hex(SHA512("")) for empty body
signString = method + "\n" + 
             requestPath + "\n" +             // path without host, with leading /
             queryString + "\n" +              // raw, NOT URL-encoded; "" if none
             hashedBody + "\n" + 
             timestamp                          // Unix seconds as string

sign = hex(HMAC-SHA512(signString, secret))
```

Timestamp deviation tolerance: 15 minutes. Generous, but still sync.

The empty-body SHA-512:
`cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e`

## Error semantics

Success: HTTP 2xx + JSON body.

Errors: HTTP 4xx/5xx + body like:

```json
{
  "label": "INVALID_KEY",
  "message": "Invalid API key provided",
  "detail": "..."
}
```

Map by `label`:

| Label | Map to |
|---|---|
| `INVALID_KEY` / `INVALID_SIGNATURE` / `MISSING_REQUIRED_HEADER` | `ErrAuth` |
| `REQUEST_EXPIRED` | re-sync, retry once |
| `TOO_MANY_REQUESTS` | `ErrRateLimit` |

## Positions: `GET /api/v4/futures/usdt/positions`

Response (array; only non-zero by default):

```json
[
  {
    "user": 10000,
    "contract": "BTC_USDT",
    "size": 150,                       // signed, in contracts not coin
    "leverage": "10",
    "risk_limit": "1000000",
    "leverage_max": "100",
    "maintenance_rate": "0.005",
    "value": "9601.84",                // notional in USDT
    "margin": "960.18",
    "entry_price": "63421.5",
    "liq_price": "59421.5",
    "mark_price": "64012.30",
    "unrealised_pnl": "88.62",
    "realised_pnl": "0",
    "history_pnl": "0",
    "last_close_pnl": "0",
    "realised_point": "0",
    "history_point": "0",
    "adl_ranking": 5,
    "pending_orders": 0,
    "close_order": null,
    "mode": "single",                  // or "dual_long" / "dual_short"
    "cross_leverage_limit": "10"
  }
]
```

### Gate quirks

- **`size` is in number of contracts, not coin units.** For USDT-M perpetuals,
  each symbol has a `quanto_multiplier` (e.g., `BTC_USDT` is 0.0001 BTC per
  contract). To convert: `coinSize = contracts * quanto_multiplier`.

  Fetch multiplier from `/api/v4/futures/usdt/contracts/{contract}` and cache
  per-symbol on the client. Alternatively, we can leave `Size` as contracts
  and clearly label in UI — but that breaks cross-exchange consistency.

  **Decision for M4**: cache `quanto_multiplier` per symbol on first
  encounter. Convert size to coin units before returning. Document this.

- **`size` is signed.** Negative = short. Same handling as Binance.

- **Symbol format**: `BTC_USDT`. Strip `_` → `BTCUSDT`.

### Mapping

```go
size := parseFloat(item.Size)  // contracts, possibly negative
if size == 0 { continue }

mult := c.getMultiplier(ctx, item.Contract)  // 0.0001 for BTC_USDT
coinSize := math.Abs(size) * mult

side := SideLong
if size < 0 { side = SideShort }

sym := strings.ReplaceAll(item.Contract, "_", "")

p := Position{
    Exchange:      "gate",
    Symbol:        sym,
    RawSymbol:     item.Contract,
    Side:          side,
    Size:          coinSize,
    EntryPrice:    parseFloat(item.EntryPrice),
    MarkPrice:     parseFloat(item.MarkPrice),
    UnrealizedPnL: parseFloat(item.UnrealisedPnl),
    Leverage:      parseFloat(item.Leverage),
    Notional:      parseFloat(item.Value),
    MarginMode:    "cross",  // Gate exposes mode at user level; assume cross
    UpdatedAt:     time.Now(),  // Gate doesn't return per-position updated_time
}
```

## AvailableBalance: `GET /api/v4/futures/usdt/accounts`

Response (single object, not array):

```json
{
  "total": "10362.92",
  "available": "10000.00",
  "unrealised_pnl": "12.50",
  "position_margin": "350.42",
  "order_margin": "0",
  "point": "0",
  "currency": "USDT",
  "in_dual_mode": false,
  "enable_credit": false,
  "position_initial_margin": "350.42",
  "maintenance_margin": "10.00"
}
```

Return `available`.

## History: `GET /api/v4/futures/usdt/position_close?limit=100`

Response (array):

```json
[
  {
    "time": 1684742410,                // Unix seconds
    "pnl": "88.61",
    "side": "long",
    "contract": "BTC_USDT",
    "text": "..."
  }
]
```

**Lean response**: only time, pnl, side, contract. Map to `ClosedPosition`
with everything else zero/empty.

```go
cp := ClosedPosition{
    Exchange:    "gate",
    Symbol:      strings.ReplaceAll(item.Contract, "_", ""),
    Side:        sideFromString(item.Side),
    RealizedPnL: parseFloat(item.Pnl),
    CloseTime:   time.Unix(item.Time, 0),
    // Size, EntryPrice, ExitPrice, OpenTime: zero/empty
}
```

## ClosePosition: `POST /api/v4/futures/usdt/orders` with `size=0, close=true`

Gate's idiom is unusual: to close a position, submit an order with `size: 0`
and `close: true`. The server determines the close direction from current
position.

Body:

```json
{
  "contract": "BTC_USDT",
  "size": 0,
  "price": "0",
  "tif": "ioc",
  "close": true,
  "reduce_only": true
}
```

`tif: "ioc"` and `price: "0"` together make it a market order in Gate.

Hedge mode (`mode: dual_long` / `dual_short`): pass `auto_size: "close_long"`
or `auto_size: "close_short"` instead of relying on `close: true`. For M4,
detect via `Position.MarginMode` proxy — but Gate's `mode` is per-position
not per-account, so we need to keep `mode` from the read on the position
record.

**Decision for M4**: pass `close=true` and `reduce_only=true`; on one-way mode
this works. For hedge mode (`mode != "single"`), pass `auto_size` appropriately.
Stash `mode` on a Gate-specific wrapper on Position.

### Response

```json
{
  "id": 1234567890,
  "contract": "BTC_USDT",
  "size": 0,
  "status": "open",
  ...
}
```

`id` is a number; format as string for `CloseResult.OrderID`.

## File layout

```
internal/exchange/gate/
  client.go     // signing with SHA-512 and Unix-seconds timestamp, do()
  positions.go
  balance.go
  history.go
  close.go
  contracts.go  // multiplier cache
  types.go
  *_test.go
```

## Testing checklist

- [ ] `TestSign_UsesSHA512` — explicit fixture; failing this means accidental SHA-256
- [ ] `TestSign_UnixSecondsTimestamp` — verify timestamp string fits `^\d{10}$`
- [ ] `TestSign_EmptyBodyUsesEmptyStringHash` — verify SHA512("") in payload
- [ ] `TestPositions_ContractToCoinConversion` — 150 contracts of BTC_USDT (mult 0.0001) → 0.015 coin
- [ ] `TestPositions_NegativeSizeIsShort`
- [ ] `TestBalance_ReturnsAvailable`
- [ ] `TestClose_PassesCloseTrue`
- [ ] `TestClose_HedgeUsesAutoSize`

## Pitfalls collected

- **SHA-512, not SHA-256.** Test must verify the hex string is 128 chars, not 64.
- **Unix seconds, not milliseconds.** Other adapters use ms; don't copy-paste.
- **`size` is in contracts, not coin.** Multiplier lookup required.
- **`time` field in history is seconds, not ms.** Cast with `time.Unix(n, 0)`.
- **`auto_size` only in hedge mode.** Passing it in one-way returns an error.
