# 03 — Bitget V2 Mix (USDT-M Futures) adapter

## Endpoints

| Function | Method | Path | Notes |
|---|---|---|---|
| Server time | `GET` | `/api/v2/public/time` | Public |
| Positions | `GET` | `/api/v2/mix/position/all-position` | `productType=USDT-FUTURES` |
| Account | `GET` | `/api/v2/mix/account/accounts` | `productType=USDT-FUTURES` |
| History positions | `GET` | `/api/v2/mix/position/history-position` | Native |
| Flash close | `POST` | `/api/v2/mix/order/close-positions` | Native single-step |

**Base URL**: `https://api.bitget.com`

No separate testnet domain. Bitget has demo coins (`SUSDT`, `SBTC`, …) instead;
treat as out of scope.

## Authentication

Headers:

| Header | Value |
|---|---|
| `ACCESS-KEY` | API key |
| `ACCESS-SIGN` | base64 of HMAC-SHA256 |
| `ACCESS-PASSPHRASE` | Passphrase set at key creation |
| `ACCESS-TIMESTAMP` | Unix milliseconds (number as string) |
| `locale` | `en-US` (optional but recommended) |
| `Content-Type` | `application/json` for POST |

**Signing**:

```
preHash = timestamp + method + requestPath + body
        method:     "GET" / "POST" (uppercase)
        requestPath: includes query string for GET, e.g.
                     "/api/v2/mix/position/all-position?productType=USDT-FUTURES&marginCoin=USDT"
        body:       "" for GET, raw JSON for POST

sign = base64(HMAC-SHA256(preHash, secret))
```

Very similar to OKX, but timestamp is Unix milliseconds (string), not ISO.

## Error semantics

```json
{
  "code": "00000",
  "msg": "success",
  "requestTime": 1659076670000,
  "data": ...
}
```

`code` is string. `"00000"` = success. Common codes:

| Code | Meaning | Map to |
|---|---|---|
| 40009 | Invalid signature | `ErrAuth` |
| 40010 | Invalid timestamp | re-sync, retry once |
| 40006 | Invalid API key | `ErrAuth` |
| 40015 | Invalid passphrase | `ErrAuth` |
| 429xx | Rate limit | `ErrRateLimit` |

## Positions: `GET /api/v2/mix/position/all-position`

Query: `productType=USDT-FUTURES&marginCoin=USDT`

Response:

```json
{
  "code": "00000",
  "data": [
    {
      "symbol": "BTCUSDT",
      "marginCoin": "USDT",
      "holdSide": "long",            // "long" or "short"
      "openDelegateSize": "0",
      "marginSize": "960.18",
      "available": "0.150",
      "locked": "0.000",
      "total": "0.150",              // total position size
      "leverage": "10",
      "achievedProfits": "0",
      "openPriceAvg": "63421.5",
      "marginMode": "isolated",      // "isolated" or "crossed"
      "posMode": "one_way_mode",     // or "hedge_mode"
      "unrealizedPL": "88.62",
      "liquidationPrice": "59421.5",
      "keepMarginRate": "0.005",
      "markPrice": "64012.30",
      "cTime": "1720700000000",
      "uTime": "1720736417660"
    }
  ]
}
```

### Mapping

```go
total := parseFloat(item.Total)
if total == 0 { continue }

side := SideLong
if item.HoldSide == "short" {
    side = SideShort
}

markMode := "cross"
if item.MarginMode == "isolated" {
    markMode = "isolated"
}

p := Position{
    Exchange:      "bitget",
    Symbol:        item.Symbol,
    RawSymbol:     item.Symbol,
    Side:          side,
    Size:          total,
    EntryPrice:    parseFloat(item.OpenPriceAvg),
    MarkPrice:     parseFloat(item.MarkPrice),
    UnrealizedPnL: parseFloat(item.UnrealizedPL),
    Leverage:      parseFloat(item.Leverage),
    Notional:      total * parseFloat(item.MarkPrice),  // not provided directly
    MarginMode:    markMode,
    UpdatedAt:     time.UnixMilli(parseInt(item.UTime)),
}
```

Note: Bitget doesn't return a notional field; compute as `size × markPrice`.

## AvailableBalance: `GET /api/v2/mix/account/accounts?productType=USDT-FUTURES`

Response:

```json
{
  "code": "00000",
  "data": [{
    "marginCoin": "USDT",
    "locked": "0.31",
    "available": "10000.00",
    "crossedMaxAvailable": "10580.56",
    "isolatedMaxAvailable": "10580.56",
    "maxTransferOut": "10572.92",
    "accountEquity": "10582.90",
    "usdtEquity": "10582.90",
    "unrealizedPL": "",
    "assetMode": "single"
  }]
}
```

Find entry where `marginCoin == "USDT"`, return `available`.

## History: `GET /api/v2/mix/position/history-position?productType=USDT-FUTURES`

```json
{
  "code": "00000",
  "data": {
    "list": [{
      "positionId": "...",
      "marginCoin": "USDT",
      "symbol": "BTCUSDT",
      "holdSide": "long",
      "openAvgPrice": "63421.5",
      "closeAvgPrice": "64012.27",
      "marginMode": "isolated",
      "openTotalPos": "0.15",
      "closeTotalPos": "0.15",
      "pnl": "88.61",                  // gross PnL
      "netProfit": "88.31",            // after fees
      "totalFunding": "0.05",
      "openFee": "-0.15",
      "closeFee": "-0.15",
      "posMode": "one_way_mode",
      "ctime": "1684700000000",        // open time
      "utime": "1684742410020"         // close time
    }],
    "endId": "..."
  }
}
```

Use `netProfit` for `ClosedPosition.RealizedPnL` (post-fee, consistent with what
the user actually saw in their wallet).

Default range: last 3 months. Supports paging via `endId` cursor.

## ClosePosition: `POST /api/v2/mix/order/close-positions`

Body:

```json
{
  "symbol": "BTCUSDT",
  "productType": "USDT-FUTURES",
  "holdSide": "long"
}
```

`holdSide` required only in hedge mode. In one-way mode, can omit (or pass
anyway, harmless).

Decision for M4: always pass `holdSide` mapped from `req.Side`.

### Response

```json
{
  "code": "00000",
  "data": {
    "successList": [{
      "orderId": "1234567890",
      "clientOid": "..."
    }],
    "failureList": []
  }
}
```

Map first entry of `successList[0].orderId` to `CloseResult.OrderID`. If
`failureList` is non-empty, return an error.

## File layout

```
internal/exchange/bitget/
  client.go     // signing, time sync, do()
  positions.go
  balance.go
  history.go
  close.go
  types.go
  *_test.go
```

## Testing checklist

- [ ] `TestSign_GETIncludesFullPathWithQuery` — fixture from docs
- [ ] `TestSign_POSTIncludesBody`
- [ ] `TestPositions_SkipsZeroTotal`
- [ ] `TestPositions_NotionalComputed` — Notional = size * markPrice
- [ ] `TestBalance_FindsUSDTEntry`
- [ ] `TestClose_PassesHoldSide`
- [ ] `TestClose_FailureListProducesError`

## Pitfalls collected

- **`code` is string `"00000"`**, not `"0"` or `0`. Five zeros.
- **Position has no notional field.** Compute it. Don't try to find it.
- **`marginMode` value is `"crossed"` (with -ed) on some endpoints**, `"cross"`
  on others. Normalize when mapping.
- **Demo trading uses different product type (`SUSDT-FUTURES`).** We don't
  support it; ignore.
