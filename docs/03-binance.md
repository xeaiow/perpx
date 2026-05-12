# 03 — Binance USDⓈ-M Futures adapter

## Endpoints

| Function | Method | Path | Notes |
|---|---|---|---|
| Server time | `GET` | `/fapi/v1/time` | Public |
| Positions | `GET` | `/fapi/v3/positionRisk` | v3 is the current version |
| Balance | `GET` | `/fapi/v3/balance` | Array of asset balances |
| Income history | `GET` | `/fapi/v1/income` | Used to derive history |
| User trades | `GET` | `/fapi/v1/userTrades` | Used to attach prices to history |
| Place order | `POST` | `/fapi/v1/order` | For close-position |

**Base URL**:
- Mainnet: `https://fapi.binance.com`
- Testnet: `https://testnet.binancefuture.com`

## Authentication

Header: `X-MBX-APIKEY: <apiKey>`

Signed requests append two parameters to the query string (or body for POST):

```
timestamp=<ms>
signature=<hex>
```

**Signing**:

```
totalParams = full query string OR full URL-encoded body (whichever you're sending)
            (must include timestamp, recvWindow if set)
signature   = hex(HMAC-SHA256(totalParams, apiSecret))
```

Then append `&signature=<hex>` to the same place (query for GET, body for POST).

`recvWindow=5000` recommended.

## Error semantics

Status code 200 means success. Errors return JSON like:

```json
{"code": -1021, "msg": "Timestamp for this request is outside of the recvWindow."}
```

Map common codes:

| Code | Meaning | Map to |
|---|---|---|
| -1021 | Timestamp outside recv window | re-sync, retry once |
| -1022 | Invalid signature | `ErrAuth` |
| -2014/-2015 | Bad API key / IP / permissions | `ErrAuth` |
| -1003 | Too many requests | `ErrRateLimit` |

HTTP 429/418 also → `ErrRateLimit`.

## Positions: `GET /fapi/v3/positionRisk`

Response (array; only symbols with positions or open orders):

```json
[
  {
    "symbol": "BTCUSDT",
    "positionSide": "BOTH",          // BOTH (one-way), LONG, SHORT (hedge)
    "positionAmt": "0.150",           // signed: positive=long, negative=short
    "entryPrice": "63421.5",
    "breakEvenPrice": "63489.7",
    "markPrice": "64012.30",
    "unRealizedProfit": "88.62",
    "liquidationPrice": "59421.5",
    "leverage": "10",
    "marginType": "cross",            // "cross" or "isolated"
    "isolatedMargin": "0.0",
    "notional": "9601.84",
    "marginAsset": "USDT",
    "updateTime": 1720736417660
  }
]
```

### Mapping

```go
amt := parseFloat(item.PositionAmt)
if amt == 0 { continue }

side := SideLong
if amt < 0 {
    side = SideShort
}

p := Position{
    Exchange:      "binance",
    Symbol:        item.Symbol,
    RawSymbol:     item.Symbol,
    Side:          side,
    Size:          math.Abs(amt),
    EntryPrice:    parseFloat(item.EntryPrice),
    MarkPrice:     parseFloat(item.MarkPrice),
    UnrealizedPnL: parseFloat(item.UnRealizedProfit),
    Leverage:      parseFloat(item.Leverage),
    Notional:      math.Abs(parseFloat(item.Notional)),
    MarginMode:    item.MarginType,
    UpdatedAt:     time.UnixMilli(item.UpdateTime),
}
```

### Hedge mode

`positionSide=BOTH` is one-way. `LONG` and `SHORT` are hedge mode.

For our purposes, we expose hedge positions as **two separate rows** in the
Positions tab — already the right call because each must be closed individually.

The `positionSide` value must be passed back on close.

## AvailableBalance: `GET /fapi/v3/balance`

Response:

```json
[
  {
    "accountAlias": "...",
    "asset": "USDT",
    "balance": "12450.32",
    "crossWalletBalance": "12450.32",
    "crossUnPnl": "0.00",
    "availableBalance": "12450.32",
    "maxWithdrawAmount": "12450.32",
    "marginAvailable": true,
    "updateTime": 1617939110373
  },
  // BNB, ETH, ... possibly
]
```

Find the entry where `asset == "USDT"`, return `availableBalance`.

If none found, return `0, nil`. (User may have no USDT-M balance.)

## History: aggregated from `/fapi/v1/income`

Binance **has no native closed-positions endpoint**. We approximate from income:

Query: `incomeType=REALIZED_PNL&limit=1000&startTime=<ms>`

Response (array):

```json
[
  {
    "symbol": "BTCUSDT",
    "incomeType": "REALIZED_PNL",
    "income": "88.61",
    "asset": "USDT",
    "info": "",
    "time": 1684742410020,
    "tranId": 9689322392,
    "tradeId": "2059192"
  }
]
```

### Compromise design for M4

For M4 we ship a **simplified** history that just shows each `REALIZED_PNL`
entry as a `ClosedPosition` with:

- `Symbol`: from income
- `RealizedPnL`: `parseFloat(income)`
- `CloseTime`: `time.UnixMilli(item.Time)`
- `Side`, `Size`, `EntryPrice`, `ExitPrice`, `OpenTime`: **empty/zero**

UI will tolerate missing fields and show a `—` for them. Acceptable because
this surface is informational, not actionable.

A full implementation would cross-reference `userTrades` to derive entry/exit
prices and times. Defer that to a future iteration; mark it clearly in code.

## ClosePosition: `POST /fapi/v1/order`

Body (form-encoded; not JSON):

```
symbol=BTCUSDT
side=SELL                      # opposite of position side
type=MARKET
quantity=0.150
reduceOnly=true                # for one-way mode
positionSide=BOTH              # one-way; LONG/SHORT in hedge
timestamp=<ms>
signature=<hex>
```

### One-way vs hedge

| Position came in with | side to send | positionSide to send | reduceOnly |
|---|---|---|---|
| `positionSide=BOTH`, `Side=Long` | `SELL` | `BOTH` | `true` |
| `positionSide=BOTH`, `Side=Short` | `BUY` | `BOTH` | `true` |
| `positionSide=LONG` (hedge) | `SELL` | `LONG` | (omit; reduceOnly invalid here) |
| `positionSide=SHORT` (hedge) | `BUY` | `SHORT` | (omit) |

**Critical**: hedge mode with `reduceOnly=true` returns error -4061
("Order's position side does not match user's setting"). Adapter must track
the original `positionSide` from the read and route accordingly.

Stash the `positionSide` in a Binance-specific wrapper on `Position` so it
survives through `CloseRequest`. Or: re-fetch position info before submitting
to determine the right shape.

### Response

```json
{
  "orderId": 28,
  "symbol": "BTCUSDT",
  "status": "NEW",
  ...
}
```

Map `orderId` (number) → `CloseResult.OrderID` (string via `strconv.FormatInt`).

## File layout

```
internal/exchange/binance/
  client.go        // signing, time sync, do()
  positions.go
  balance.go
  history.go       // income-based aggregation
  close.go         // form-encoded order placement
  types.go
  *_test.go
```

## Testing checklist

Standard set (signing fixture, time sync, happy/error path for each method) plus:

- [ ] `TestPositions_HedgeModeTwoRows` — same symbol with LONG and SHORT both
  surface as separate `Position` entries
- [ ] `TestPositions_NegativeAmtIsShort` — `positionAmt=-0.5` → `Side=Short`, `Size=0.5`
- [ ] `TestBalance_FindsUSDTAmongMultipleAssets`
- [ ] `TestHistory_MapsRealizedPnlEntries` — three REALIZED_PNL entries → three `ClosedPosition`
- [ ] `TestClose_OneWayMode` — sends `reduceOnly=true&positionSide=BOTH`
- [ ] `TestClose_HedgeLong` — sends `positionSide=LONG`, omits `reduceOnly`
- [ ] `TestClose_FormEncoded` — body is `application/x-www-form-urlencoded`, not JSON

## Pitfalls collected

- **POST body is form-encoded.** Not JSON. Content-Type must be
  `application/x-www-form-urlencoded`. Sign the form-encoded body.
- **`positionAmt` is signed.** Other exchanges return absolute value. We normalize
  to absolute + `Side`.
- **`/fapi/v3/positionRisk` only returns symbols with positions or open orders.**
  Empty wallet still gets an empty array, not all symbols.
- **Symbol "BTCUSDT" vs "BTCUSDC"**: spec says USDT-M only. Filter by suffix or
  by `marginAsset == "USDT"` if mixing.
