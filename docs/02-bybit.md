# 02 — Bybit V5 adapter

Implement first. Zoomex (M3) extends this adapter, so getting the design clean
here saves time on the next step.

## Endpoints

| Function | Method | Path | Notes |
|---|---|---|---|
| Server time | `GET` | `/v5/market/time` | Public; for time-sync |
| Positions | `GET` | `/v5/position/list` | `category=linear` + `settleCoin=USDT` |
| Wallet balance | `GET` | `/v5/account/wallet-balance` | `accountType=UNIFIED` |
| Closed PnL (history) | `GET` | `/v5/position/closed-pnl` | `category=linear` |
| Place order (for close) | `POST` | `/v5/order/create` | reduce-only market order |

**Base URL**:
- Mainnet: `https://api.bybit.com`
- Testnet: `https://api-testnet.bybit.com`

Pick from `cfg.Runtime.UseTestnet`.

## Authentication

Headers required on every private request:

| Header | Value |
|---|---|
| `X-BAPI-API-KEY` | API key |
| `X-BAPI-TIMESTAMP` | Unix milliseconds, adjusted by server-time delta |
| `X-BAPI-RECV-WINDOW` | `5000` (ms) |
| `X-BAPI-SIGN` | HMAC-SHA256 hex; see formula below |

**Signing formula**:

```
signaturePayload = timestamp + apiKey + recvWindow + queryStringOrJsonBody
signature        = hex(HMAC-SHA256(signaturePayload, apiSecret))
```

- For `GET`: queryStringOrJsonBody is the raw query string (no leading `?`),
  with parameters in **the same order as in the URL**.
- For `POST`: queryStringOrJsonBody is the JSON body, **byte-exact** as sent.
  Serialize once, sign that bytes, send those bytes.

### Worked example (from Bybit docs)

```
timestamp     = "1672382213281"
apiKey        = "VPm8wWcYAhERmkPC83"
recvWindow    = "5000"
queryString   = "accountType=UNIFIED"
secret        = "rh2H1HhJWslsl0dG2W1QxvxsM1ulfPLb3LMq"

payload  = "1672382213281VPm8wWcYAhERmkPC835000accountType=UNIFIED"
sign     = hex(HMAC-SHA256(payload, secret))
         = "f3148e5fe6e1bb1a6c5b1b0c5b1f0c5b1f0c5b1f0c5b1f0c5b1f0c5b1f0c5b1f"  // placeholder
```

Use this as the unit-test fixture. The actual expected signature for fixture
values is computable; test computes and compares to a hardcoded expected value
that future Claude Code sessions can verify.

## Response envelope

All endpoints return:

```json
{
  "retCode": 0,
  "retMsg": "OK",
  "result": { ... },
  "retExtInfo": {},
  "time": 1672382213281
}
```

`retCode == 0` means success. Otherwise read `retMsg`. Common codes:

| Code | Meaning | Map to |
|---|---|---|
| 10001 | Parameter error | `ErrClientSide` |
| 10002 | Timestamp out of recv_window | re-sync time, then retry once |
| 10003 | Invalid API key | `ErrAuth` |
| 10004 | Bad signature | `ErrAuth` |
| 10005 | Permissions denied | `ErrAuth` |
| 10006 | Too many visits | `ErrRateLimit` |

## Positions: `GET /v5/position/list`

Query: `category=linear&settleCoin=USDT`

Response result:

```json
{
  "list": [
    {
      "symbol": "BTCUSDT",
      "side": "Buy",                 // "Buy" or "Sell"; "" means flat
      "size": "0.150",                // absolute value already
      "positionValue": "9601.84",     // notional = size * markPrice
      "avgPrice": "63421.5",
      "markPrice": "64012.3",
      "leverage": "10",
      "unrealisedPnl": "88.62",       // note British spelling
      "tradeMode": 0,                 // 0=cross, 1=isolated (classic) — see below
      "positionIdx": 0,               // 0=one-way, 1=hedge long, 2=hedge short
      "createdTime": "1676538056258",
      "updatedTime": "1684742400015"
    }
  ],
  "category": "linear",
  "nextPageCursor": ""
}
```

### Mapping

```go
p := Position{
    Exchange:      "bybit",
    Symbol:        item.Symbol,                 // already BTCUSDT
    RawSymbol:     item.Symbol,
    Side:          sideFromBybit(item.Side),    // "Buy"→Long, "Sell"→Short
    Size:          parseFloat(item.Size),
    EntryPrice:    parseFloat(item.AvgPrice),
    MarkPrice:     parseFloat(item.MarkPrice),
    UnrealizedPnL: parseFloat(item.UnrealisedPnl),
    Leverage:      parseFloat(item.Leverage),
    Notional:      parseFloat(item.PositionValue),
    MarginMode:    marginModeFromBybit(item.TradeMode),
    UpdatedAt:     time.UnixMilli(parseInt(item.UpdatedTime)),
}
if p.Size == 0 { continue }
```

### Unified account caveat

UTA (Unified Trading Account) treats margin as cross-by-default; `tradeMode`
may be `0` regardless of actual mode. For now, just report `"cross"` for UTA
and label classic-account isolated correctly. UTA-vs-classic detection is out
of scope for M2; document the limitation and move on.

## AvailableBalance: `GET /v5/account/wallet-balance`

Query: `accountType=UNIFIED&coin=USDT`

Response:

```json
{
  "list": [{
    "accountType": "UNIFIED",
    "coin": [{
      "coin": "USDT",
      "walletBalance": "10350.42",
      "availableToWithdraw": "10000.00",
      "unrealisedPnl": "12.50",
      "equity": "10362.92"
    }]
  }]
}
```

Return `availableToWithdraw` for the USDT entry. If the user uses classic
(non-UTA) account, the call returns `accountType: CONTRACT` instead; try
`UNIFIED` first, then fall back to `CONTRACT` on a specific error code.

```go
// Try UNIFIED first
bal, err := c.fetchBalance(ctx, "UNIFIED")
if errors.Is(err, errAccountTypeUnsupported) {
    bal, err = c.fetchBalance(ctx, "CONTRACT")
}
```

## History: `GET /v5/position/closed-pnl`

Query: `category=linear&limit=200&startTime=<ms>`

Response:

```json
{
  "list": [{
    "symbol": "BTCUSDT",
    "side": "Buy",                    // direction of the *closing* fill
    "qty": "0.15",
    "orderPrice": "64000.0",
    "orderType": "Market",
    "execType": "Trade",
    "closedSize": "0.15",
    "cumEntryValue": "9513.225",
    "avgEntryPrice": "63421.5",
    "cumExitValue": "9601.84",
    "avgExitPrice": "64012.27",
    "closedPnl": "88.61",
    "fillCount": "1",
    "leverage": "10",
    "createdTime": "1684742400015",
    "updatedTime": "1684742410020"
  }]
}
```

### Side mapping (subtle)

`side` is the side of the **closing** fill. The position itself is the
opposite. So:

```go
positionSide := SideShort
if item.Side == "Sell" {
    positionSide = SideLong  // sold to close means we were long
}
```

### Time range

Bybit allows up to **7 days** by default, max 2 years with paging. For the
History tab, default to the last 7 days. Allow `since` parameter to override.

## ClosePosition: `POST /v5/order/create`

Bybit has no dedicated close endpoint. Submit a market order with `reduceOnly=true`.

Body:

```json
{
  "category": "linear",
  "symbol": "BTCUSDT",
  "side": "Sell",                  // opposite of position side
  "orderType": "Market",
  "qty": "0.15",
  "reduceOnly": true,
  "positionIdx": 0                  // 0 for one-way; 1 for hedge-long; 2 for hedge-short
}
```

### Side flip

```go
side := "Sell"
if req.Side == SideShort {
    side = "Buy"
}
```

### Hedge mode

If the position came from a `positionIdx != 0`, we must pass the matching
`positionIdx`. Stash this on `Position` if needed — but for the close API, the
adapter can fetch the current position info just before submitting to get the
right `positionIdx`. Simpler: thread `positionIdx` through `Position.Raw`
metadata, or store it on a Bybit-specific wrapper.

**Decision for M2**: pass `positionIdx=0` (one-way). Hedge mode support is
deferred; document the limitation and emit an error if the user has hedge-mode
positions: "Bybit hedge mode not supported yet."

### Response

```json
{
  "retCode": 0,
  "result": {
    "orderId": "1321003749386152448",
    "orderLinkId": "..."
  }
}
```

Map to `CloseResult{OrderID: result.orderId, ...}`. **Do not** poll for fill
status here — the UI's refresh after close will pick up the new position state.

## File layout

```
internal/exchange/bybit/
  client.go      // type Client, signing, do(), time sync
  positions.go   // GetPositions implementation
  balance.go     // AvailableBalance
  history.go     // ClosedPnL
  close.go       // ClosePosition via reduce-only order
  types.go       // raw response structs (unexported)
  client_test.go // signing fixture, time-sync logic
  *_test.go      // httptest-based tests per endpoint
```

`client.go` skeleton:

```go
package bybit

type Client struct {
    apiKey    []byte
    apiSecret []byte
    baseURL   string
    http      *http.Client
    recvWindow string

    mu        sync.Mutex
    timeDelta time.Duration // server - local
    synced    bool
}

func New(c *config.Credentials, rt config.Runtime) *Client {
    baseURL := "https://api.bybit.com"
    if rt.UseTestnet {
        baseURL = "https://api-testnet.bybit.com"
    }
    return &Client{
        apiKey:     c.APIKey,
        apiSecret:  c.APISecret,
        baseURL:    baseURL,
        recvWindow: "5000",
        http:       &http.Client{Timeout: time.Duration(rt.HTTPTimeoutSec) * time.Second},
    }
}

func (c *Client) Name() string { return "bybit" }

// do performs a signed request. method is GET or POST.
// For GET: pass query as url.Values; body must be nil.
// For POST: pass body as []byte (already-marshaled JSON); query must be nil.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, out any) error {
    // 1. Ensure time synced
    // 2. Build timestamp string (server-time-adjusted)
    // 3. Build sign payload: timestamp + apiKey + recvWindow + (query.Encode() or string(body))
    // 4. Sign with HMAC-SHA256, hex encode
    // 5. Build request with headers
    // 6. Execute; check HTTP status; parse envelope; check retCode
    // 7. Unmarshal result into out
}
```

## Testing checklist

- [ ] `TestSign_KnownFixture` — payload from this doc → expected hex
- [ ] `TestSign_QueryOrderMatters` — confirms params ordered as in URL
- [ ] `TestTimeSync_AppliesDelta` — mock server time 5s ahead; verify ts uses server time
- [ ] `TestTimeSync_RetryOnRecvWindowError` — first call gets 10002, re-syncs, retries once
- [ ] `TestPositions_Happy` — fixture response, two longs + one short, one empty filtered out
- [ ] `TestPositions_Normalization` — Side=Buy → Long, Size always positive
- [ ] `TestPositions_AuthError` — retCode=10003 → `ErrAuth`
- [ ] `TestBalance_UnifiedSuccess`
- [ ] `TestBalance_FallbackToContract`
- [ ] `TestClose_SideFlip` — long → Sell, short → Buy
- [ ] `TestClose_ReduceOnlyTrue` — request body contains `"reduceOnly":true`
- [ ] `TestHistory_SideMapping` — closing fill side flipped to original position side
- [ ] `TestHistory_DefaultSince` — when `since` is zero, request omits `startTime` or uses now-7d

Optional, env-gated:
- [ ] `TestIntegration_Testnet` — gated by `INTEGRATION_BYBIT=1` + testnet keys

## Pitfalls collected

- **Spelling**: Bybit uses British "unrealised" not "unrealized". Don't typo.
- **JSON body must be byte-identical** between sign and send. Marshal once.
  Do not re-marshal after signing.
- **Query string encoding**: don't URL-encode keys/values that the docs show
  unencoded. Bybit's example signs raw `accountType=UNIFIED`, not
  `accountType%3DUNIFIED`. Use `url.Values.Encode()` and verify the output
  matches what you send in the URL.
- **`positionIdx` is required for hedge mode** even on read endpoints sometimes.
  We stick to one-way for M2.
- **Empty positions** come back with `size: "0"`, `side: ""`. Filter both.
