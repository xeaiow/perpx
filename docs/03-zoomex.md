# 03 — Zoomex V3 adapter (Bybit V5 fork)

Zoomex is a fork of Bybit V5. The signing, headers, request/response shapes,
and enum values are byte-identical. **Only the path prefix and base URL differ.**

**Build this only after Bybit (M2) is done.** This adapter wraps/embeds the
Bybit client.

## Differences from Bybit

| Aspect | Bybit | Zoomex |
|---|---|---|
| Base URL | `https://api.bybit.com` | `https://openapi.zoomex.com` |
| Testnet URL | `https://api-testnet.bybit.com` | `https://openapi-testnet.zoomex.com` |
| Path prefix | `/v5/` | `/cloud/trade/v3/` |
| Headers | `X-BAPI-*` | `X-BAPI-*` (identical) |
| Sign formula | `ts + apiKey + recvWindow + (query OR body)` | identical |
| Response envelope | `{retCode, retMsg, result, retExtInfo, time}` | identical |
| Position list `category=linear` | yes | yes |

That's the entire delta.

## Path mapping

| Function | Bybit | Zoomex |
|---|---|---|
| Server time | `/v5/market/time` | `/cloud/trade/v3/market/time` |
| Positions | `/v5/position/list` | `/cloud/trade/v3/position/list` |
| Wallet balance | `/v5/account/wallet-balance` | `/cloud/trade/v3/account/wallet-balance` |
| Closed PnL | `/v5/position/closed-pnl` | `/cloud/trade/v3/position/close-pnl` |
| Place order | `/v5/order/create` | `/cloud/trade/v3/order/create` |

Note one tiny inconsistency: Bybit uses `closed-pnl`, Zoomex uses `close-pnl`
(no `d`). Verify per the docs at https://zoomexglobal.github.io/docs/v3/ when
implementing.

## Implementation strategy

Two viable approaches:

### Option A: Composition (preferred)

Zoomex adapter holds a `bybit.Client` configured with Zoomex base URL and a
path prefix:

```go
// internal/exchange/zoomex/client.go
package zoomex

import "github.com/yourname/poscli/internal/exchange/bybit"

type Client struct {
    inner *bybit.Client
}

func New(c *config.Credentials, rt config.Runtime) *Client {
    baseURL := "https://openapi.zoomex.com"
    if rt.UseTestnet {
        baseURL = "https://openapi-testnet.zoomex.com"
    }
    inner := bybit.NewWithBaseURL(c, rt, baseURL, "/cloud/trade/v3")
    return &Client{inner: inner}
}

func (c *Client) Name() string                                         { return "zoomex" }
func (c *Client) Positions(ctx context.Context) ([]Position, error)    { 
    ps, err := c.inner.Positions(ctx)
    // rewrite Exchange field from "bybit" to "zoomex"
    for i := range ps { ps[i].Exchange = "zoomex" }
    return ps, err
}
// ... same for AvailableBalance, History, ClosePosition
```

This requires the Bybit client to expose:
- `NewWithBaseURL(creds, rt, baseURL, pathPrefix) *Client` — constructor with overrides
- The `do()` helper to prepend `pathPrefix` to all paths

**Prefer this option.** No code duplication, clear delegation.

### Option B: Wrap by URL substitution (anti-pattern, don't do this)

Subclass-style: copy Bybit code, search-replace `/v5/` → `/cloud/trade/v3/`,
maintain both. Avoid: future Bybit bug fixes won't propagate to Zoomex.

## Required Bybit changes to support composition

In `internal/exchange/bybit/client.go`:

```go
// Before:
func New(c *config.Credentials, rt config.Runtime) *Client { ... }

// After:
func New(c *config.Credentials, rt config.Runtime) *Client {
    baseURL := "https://api.bybit.com"
    if rt.UseTestnet {
        baseURL = "https://api-testnet.bybit.com"
    }
    return NewWithBaseURL(c, rt, baseURL, "/v5")
}

func NewWithBaseURL(c *config.Credentials, rt config.Runtime, baseURL, pathPrefix string) *Client {
    return &Client{
        ...
        baseURL:    baseURL,
        pathPrefix: pathPrefix,
    }
}

// In do():
fullURL := c.baseURL + c.pathPrefix + path  // e.g. "/v5" + "/position/list"
```

Adapters then call `c.do(ctx, "GET", "/position/list", ...)` instead of
`/v5/position/list`. The prefix is configurable.

## Endpoint path corrections

Confirmed via docs:

- Closed PnL on Zoomex: `/cloud/trade/v3/position/close-pnl` (singular, no `d`)
- Wallet balance on Zoomex: `/cloud/trade/v3/account/wallet-balance`

If Bybit fixes the inconsistency someday (or vice versa), one of these will
need to diverge. Track in `docs/03-zoomex.md` and override only that one
endpoint in the Zoomex adapter.

For now: Zoomex adapter's `History` method can override the path:

```go
func (c *Client) History(ctx context.Context, since time.Time) ([]ClosedPosition, error) {
    // Same logic as Bybit, but path is "/position/close-pnl" instead of "/position/closed-pnl"
    return c.inner.HistoryAtPath(ctx, "/position/close-pnl", since)
}
```

Where `HistoryAtPath` is an exported variant on the Bybit client that takes
the path explicitly.

## File layout

```
internal/exchange/zoomex/
  client.go          // ~30 lines: thin delegating wrapper
  client_test.go     // verify base URL, prefix, exchange-name override
```

No per-method files. The wrapper is small enough.

## Testing checklist

- [ ] `TestNew_UsesZoomexBaseURL` — production
- [ ] `TestNew_UsesZoomexTestnetURL` — when `rt.UseTestnet` is true
- [ ] `TestPositions_RewritesExchangeName` — `p.Exchange == "zoomex"`, not "bybit"
- [ ] `TestHistory_UsesClosePnlPath` — verify the singular variant

Integration test: same shape as Bybit, gated by `INTEGRATION_ZOOMEX=1`.

## Pitfalls collected

- **Don't duplicate Bybit's signing code.** Composition only.
- **Watch the `closed-pnl` vs `close-pnl` naming.** Verify against the docs
  when you write the test fixture.
- **Exchange-name override**: every `Position` returned has its `Exchange`
  field rewritten from "bybit" to "zoomex". Easy to forget if you delegate
  blindly. Test catches this.
