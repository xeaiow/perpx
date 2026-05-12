# 03 — OKX V5 adapter

## Endpoints

| Function | Method | Path | Notes |
|---|---|---|---|
| Server time | `GET` | `/api/v5/public/time` | Public |
| Positions | `GET` | `/api/v5/account/positions` | `instType=SWAP` |
| Balance | `GET` | `/api/v5/account/balance` | All currencies |
| Closed positions | `GET` | `/api/v5/account/positions-history` | Native endpoint |
| Close position | `POST` | `/api/v5/trade/close-position` | Native single-step close |

**Base URL**: `https://www.okx.com` (no separate testnet host; demo trading uses
the same host with a header flag).

For testnet (demo trading), add header `x-simulated-trading: 1`.

## Authentication

Headers on every private request:

| Header | Value |
|---|---|
| `OK-ACCESS-KEY` | API key |
| `OK-ACCESS-SIGN` | base64 of HMAC-SHA256 (see below) |
| `OK-ACCESS-TIMESTAMP` | ISO 8601 ms UTC, e.g. `2024-05-12T09:08:57.715Z` |
| `OK-ACCESS-PASSPHRASE` | Passphrase set when creating the API key |
| `Content-Type` | `application/json` (even for GET) |

**Timestamp format is unusual**: not Unix ms, but ISO 8601 with milliseconds in UTC.

```go
ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
```

**Signing**:

```
preHash = timestamp + method + requestPath + body
        method  = "GET" or "POST" (uppercase)
        requestPath includes query string for GET, e.g. "/api/v5/account/balance?ccy=USDT"
        body    = "" for GET, raw JSON string for POST

sign = base64(HMAC-SHA256(preHash, secret))
```

Note: **for GET, query string is part of `requestPath`, not appended separately.**

Server rejects requests where timestamp deviates from server time by >30s.
Sync via `/api/v5/public/time` first.

## Error semantics

```json
{
  "code": "0",
  "msg": "",
  "data": [...]
}
```

`code` is a **string**, not a number. `code == "0"` means success. Common codes:

| Code | Meaning | Map to |
|---|---|---|
| 50102 | Timestamp out of recv window | re-sync, retry once |
| 50111 | Invalid OK-ACCESS-KEY | `ErrAuth` |
| 50113 | Invalid signature | `ErrAuth` |
| 50114 | Invalid passphrase | `ErrAuth` |
| 50011 | Request too frequent | `ErrRateLimit` |

## Positions: `GET /api/v5/account/positions?instType=SWAP`

Response data is an array of position objects:

```json
{
  "code": "0",
  "data": [
    {
      "instId": "BTC-USDT-SWAP",
      "instType": "SWAP",
      "mgnMode": "cross",
      "posSide": "long",                 // "long", "short", or "net"
      "pos": "0.150",                    // signed in "net" mode; positive in long/short
      "avgPx": "63421.5",
      "markPx": "64012.30",
      "upl": "88.62",
      "lever": "10",
      "notionalUsd": "9601.84",
      "ccy": "USDT",
      "uTime": "1720736417660",
      "cTime": "1720700000000"
    }
  ]
}
```

### Mapping

```go
// Symbol: "BTC-USDT-SWAP" → "BTCUSDT"
sym := normalizeOKXSymbol(item.InstId)  // strip "-SWAP", strip "-"

pos := parseFloat(item.Pos)
if pos == 0 { continue }

var side PositionSide
switch item.PosSide {
case "long":  side = SideLong
case "short": side = SideShort
case "net":
    if pos > 0 { side = SideLong } else { side = SideShort }
}

p := Position{
    Exchange:      "okx",
    Symbol:        sym,
    RawSymbol:     item.InstId,         // keep "BTC-USDT-SWAP" for close-position
    Side:          side,
    Size:          math.Abs(pos),
    EntryPrice:    parseFloat(item.AvgPx),
    MarkPrice:     parseFloat(item.MarkPx),
    UnrealizedPnL: parseFloat(item.Upl),
    Leverage:      parseFloat(item.Lever),
    Notional:      parseFloat(item.NotionalUsd),
    MarginMode:    item.MgnMode,        // already "cross" or "isolated"
    UpdatedAt:     time.UnixMilli(parseInt(item.UTime)),
}
```

### Symbol normalization helper

```go
func normalizeOKXSymbol(instId string) string {
    s := strings.TrimSuffix(instId, "-SWAP")
    return strings.ReplaceAll(s, "-", "")
}
```

Only USDT-M perpetuals are in scope. Filter: `instType=SWAP` query already
covers this if we also check the quote is USDT. (OKX has BTC-USD-SWAP for
inverse — skip those by `ccy != "USDT"` filter, or restrict by symbol suffix.)

## AvailableBalance: `GET /api/v5/account/balance?ccy=USDT`

Response:

```json
{
  "code": "0",
  "data": [{
    "totalEq": "10362.92",
    "details": [{
      "ccy": "USDT",
      "availBal": "10000.00",
      "availEq": "10000.00",
      "cashBal": "10350.42",
      "eq": "10362.92",
      "uTime": "..."
    }]
  }]
}
```

Return `availBal` for the USDT detail (or `availEq` — both work; pick `availEq`
because it includes unrealized PnL in the available amount for cross margin).

## History: `GET /api/v5/account/positions-history?instType=SWAP`

```json
{
  "code": "0",
  "data": [{
    "instId": "BTC-USDT-SWAP",
    "posSide": "long",
    "openAvgPx": "63421.5",
    "closeAvgPx": "64012.27",
    "realizedPnl": "88.61",
    "pnl": "88.61",
    "fee": "-0.30",
    "fundingFee": "0.05",
    "openTime": "1684700000000",
    "uTime": "1684742410020",
    "closeTotalPos": "0.15"
  }]
}
```

Use `pnl` (gross) or `realizedPnl` (post-fee). Pick `realizedPnl` for
consistency with most other exchanges.

Time range: defaults to last 3 months. Pass `before`/`after` for paging.

## ClosePosition: `POST /api/v5/trade/close-position`

**OKX has a dedicated close endpoint.** Use this instead of placing a reduce-only
order — simpler and atomic.

Body:

```json
{
  "instId": "BTC-USDT-SWAP",
  "mgnMode": "cross",
  "posSide": "long",
  "ccy": "USDT"
}
```

### Required parameters

- `instId`: raw symbol from `Position.RawSymbol`
- `mgnMode`: from `Position.MarginMode` (already "cross" or "isolated")
- `posSide`: required only in **hedge mode with isolated margin**, but harmless
  to always pass when known. Map: `Long → "long"`, `Short → "short"`. For net
  mode, omit or pass `"net"`.
- `ccy`: USDT, for cross margin in spot mode (safe to always pass for USDT-M)

### Response

```json
{
  "code": "0",
  "data": [{
    "instId": "BTC-USDT-SWAP",
    "posSide": "long",
    "clOrdId": ""
  }]
}
```

**Note: no order ID returned.** Construct `CloseResult` with `OrderID: ""`. UI
must tolerate empty order IDs.

## File layout

```
internal/exchange/okx/
  client.go     // signing (note ISO 8601 timestamp), time sync, do()
  positions.go
  balance.go
  history.go    // /api/v5/account/positions-history
  close.go      // /api/v5/trade/close-position
  symbol.go     // normalize helpers
  types.go
  *_test.go
```

## Testing checklist

- [ ] `TestSign_ISOTimestamp` — verify timestamp format produces YYYY-MM-DDTHH:MM:SS.sssZ
- [ ] `TestSign_GETIncludesQueryInPath` — fixture from OKX docs
- [ ] `TestSign_POSTIncludesBodyInPreHash`
- [ ] `TestNormalizeSymbol` — `BTC-USDT-SWAP` → `BTCUSDT`
- [ ] `TestPositions_NetModeNegativeIsShort` — `posSide=net`, `pos=-0.5` → Side=Short
- [ ] `TestPositions_FilterNonUSDT` — `BTC-USD-SWAP` (inverse) entries dropped
- [ ] `TestClose_PassesMgnModePosSide`
- [ ] `TestClose_EmptyOrderID` — result has `OrderID == ""`, not an error

## Pitfalls collected

- **Timestamp is ISO 8601, not Unix ms.** This is the #1 cause of "Invalid sign".
  Format with `"2006-01-02T15:04:05.000Z"`. Note the `.000` for milliseconds.
- **`code` is a string `"0"`, not number 0.** Compare as string.
- **Demo trading uses the same host** with `x-simulated-trading: 1` header.
- **No dedicated testnet domain.** Don't try to swap base URL for testnet.
- **`Content-Type: application/json` even for GET.** OKX is picky.
- **Passphrase must match the API key's passphrase.** If user typed a different
  passphrase at key-creation time vs `poscli init`, every call returns 50114.
