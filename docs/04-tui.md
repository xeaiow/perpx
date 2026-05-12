# 04 — TUI design (Bubble Tea)

## Architecture overview

Elm-style: `Model → Update(msg) → View() → render`. I/O happens in `tea.Cmd`
returned from `Init()` or `Update()`. Never block in `View()`.

```
internal/ui/
  app.go             // root model; owns tab state; routes msgs to active tab
  tabs.go            // tab header rendering (lipgloss)
  messages.go        // tea.Msg types shared across tabs

  positions/
    model.go         // Bubble Tea model for Positions tab
    update.go        // message handling
    view.go          // table rendering
    fetch.go         // tea.Cmd to fetch positions across all exchanges in parallel

  history/
    model.go
    update.go
    view.go
    fetch.go

  accounts/
    model.go
    update.go
    view.go
    fetch.go

  confirm/
    modal.go         // close-position y/n confirmation

  styles/
    styles.go        // Lip Gloss styles centralized
```

## Layout

```
┌──────────────────────────────────────────────────────────────────────────┐
│  poscli   [ Positions ]  History   Accounts                              │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  Exchange  Symbol      Side   Size       Entry      Mark      uPnL  Lev │
│  ──────────────────────────────────────────────────────────────────────  │
│  binance   BTCUSDT     LONG   0.150     63421.5    64012.3   +88.62  10x│
│  okx       BTCUSDT     LONG   0.200     63010.0    64012.3  +200.46  20x│
│  bybit     ETHUSDT     SHORT  2.500      3120.5     3088.2   +80.75   5x│
│  bitget    SOLUSDT     LONG   100.0      142.30     138.10  -420.00   3x│
│  > gate    XRPUSDT     LONG   1000.0     0.6210     0.6311   +10.10   5x│  ← selected
│                                                                          │
│  ──────────────────────────────────────────────────────────────────────  │
│  Total uPnL: -40.07 USDT     5 positions across 5 exchanges              │
│  [↑↓] navigate  [x] close selected  [tab] switch view  [r] refresh  [q]  │
└──────────────────────────────────────────────────────────────────────────┘
```

## Tabs

| Index | Name | Component |
|---|---|---|
| 0 | Positions | `internal/ui/positions` |
| 1 | History | `internal/ui/history` |
| 2 | Accounts | `internal/ui/accounts` |

Active tab gets bold + underline + accent color. Inactive tabs are dimmed.

Switch via `tab` (forward) and `shift+tab` (backward). Numbers `1/2/3` jump
directly.

## Global key bindings (all tabs)

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Switch tab |
| `1` / `2` / `3` | Jump to tab |
| `r` | Refresh current tab |
| `q` / `ctrl+c` | Quit |
| `?` | Toggle help overlay |

## Tab-specific bindings

### Positions

| Key | Action |
|---|---|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `home` / `g` | First row |
| `end` / `G` | Last row |
| `x` | Open close-position confirmation for selected row |

### History

| Key | Action |
|---|---|
| `↑` / `↓` / `j` / `k` | Scroll |
| `pgup` / `pgdn` | Page |
| `f` | Cycle exchange filter (all → binance → okx → ...) |

### Accounts

No selection; pure info view. Refresh only.

## Confirm modal (close position)

Triggered by `x` on Positions tab.

```
┌─ Confirm Close ──────────────────────────────────┐
│                                                   │
│  Exchange:  gate                                  │
│  Symbol:    XRPUSDT                               │
│  Side:      LONG                                  │
│  Size:      1000.0                                │
│  Mark:      0.6311                                │
│  Est. PnL:  +10.10 USDT                           │
│                                                   │
│  This will submit a MARKET order to close the     │
│  position. This action is irreversible.           │
│                                                   │
│  Close this position?                              │
│                                                   │
│  [y] yes    [n / esc] cancel                      │
└───────────────────────────────────────────────────┘
```

Key handling:
- `y` → submit close, show progress, then close modal + refresh
- `n` / `esc` → cancel, close modal
- All other keys: ignored (no accidental dismissal)

## Message types

```go
// internal/ui/messages.go
package ui

import (
    "time"
    "github.com/yourname/poscli/internal/exchange"
)

// Sent after parallel fetch from all exchanges completes.
type PositionsFetchedMsg struct {
    Positions []exchange.Position
    Errors    map[string]error  // keyed by exchange name
    At        time.Time
}

type HistoryFetchedMsg struct {
    Items  []exchange.ClosedPosition
    Errors map[string]error
}

type AccountsFetchedMsg struct {
    Equity map[string]float64  // exchange name → available + Σ notional
    Errors map[string]error
}

// Result of a close-position attempt.
type CloseResultMsg struct {
    Exchange string
    Symbol   string
    Result   exchange.CloseResult
    Err      error
}

// Internal: trigger a tab change.
type SwitchTabMsg int  // 0, 1, or 2
```

## Async fetch pattern

```go
// internal/ui/positions/fetch.go
func FetchCmd(exs map[string]exchange.Exchange) tea.Cmd {
    return func() tea.Msg {
        var (
            mu       sync.Mutex
            all      []exchange.Position
            errs     = make(map[string]error)
            wg       sync.WaitGroup
        )
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()

        for name, ex := range exs {
            wg.Add(1)
            go func(name string, ex exchange.Exchange) {
                defer wg.Done()
                ps, err := ex.Positions(ctx)
                mu.Lock()
                defer mu.Unlock()
                if err != nil {
                    errs[name] = err
                    return
                }
                all = append(all, ps...)
            }(name, ex)
        }
        wg.Wait()

        return ui.PositionsFetchedMsg{
            Positions: all,
            Errors:    errs,
            At:        time.Now(),
        }
    }
}
```

Important properties:
- **Don't block** — `tea.Cmd` runs on its own goroutine; the UI stays interactive.
- **Per-exchange errors don't fail the whole fetch.** Each `Errors[name]` is
  independent. UI shows a `✗` next to that exchange's rows and a status-line message.
- **Context timeout** — 15s ceiling. Slow exchanges don't hang the UI.

## Styles

Centralize in `internal/ui/styles/styles.go`. Examples:

```go
var (
    BorderColor  = lipgloss.Color("#444444")
    AccentColor  = lipgloss.Color("#00B5D8")
    PnlPositive  = lipgloss.Color("#0AC18E")
    PnlNegative  = lipgloss.Color("#F03A47")
    Dim          = lipgloss.Color("#888888")

    TabActive   = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(AccentColor)
    TabInactive = lipgloss.NewStyle().Foreground(Dim)

    Selected    = lipgloss.NewStyle().Bold(true).Reverse(true)
    Pnl         = func(v float64) lipgloss.Style {
        if v >= 0 { return lipgloss.NewStyle().Foreground(PnlPositive) }
        return lipgloss.NewStyle().Foreground(PnlNegative)
    }
)
```

Per-exchange color tag (optional, for visual scanning):

| Exchange | Color |
|---|---|
| binance | `#F0B90B` (yellow) |
| okx     | `#000000` on light bg, `#FFFFFF` on dark — use bold instead |
| bybit   | `#F7A600` |
| bitget  | `#00B377` |
| gate    | `#1A1AFF` — replace with cyan to be readable on dark |
| zoomex  | `#FF6B00` |

These are reference colors only; pick palette that works on the dominant
terminal theme (assume dark background by default).

## Error display

Per-exchange errors live in a small status area below the table:

```
✗ okx: rate limited                ← red text
✗ gate: timeout after 15s
```

Don't pop a modal; the user is in the middle of a workflow. Persistent in the
status area until next successful fetch.

For close-position errors, show a toast (3s auto-dismiss):

```
✗ Failed to close gate:XRPUSDT — auth failed
```

## Refresh semantics

`r` triggers `FetchCmd` for the **current tab only**. Don't blanket-refresh
all tabs. Each tab tracks its own `lastFetch time.Time`; the status line
shows "Updated 23s ago" so the user knows freshness.

No automatic polling. The original spec said: manual refresh only.

## Testing

Bubble Tea testing uses `github.com/charmbracelet/x/exp/teatest`. Examples:

```go
func TestPositionsView_RendersTable(t *testing.T) {
    m := positions.New(fakeExchanges())
    tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 30))
    tm.Send(positions.FetchedMsg{Positions: sampleFixture()})

    out := readUntil(t, tm, "BTCUSDT", 2*time.Second)
    if !strings.Contains(out, "+88.62") {
        t.Errorf("expected uPnL in view")
    }
}
```

Cover at least:
- Each tab renders without panic for empty / populated / error states
- Tab switching preserves state
- Selection bounds: can't go below 0 or above row count
- Close confirm modal: `y` triggers close cmd, `n`/`esc` cancels

## Pitfalls

- **`View()` must be pure.** No I/O, no mutex grabs, no sleeps. If you need
  data, ensure it's already in the model when `View()` is called.
- **`tea.Cmd` results route through `Update`.** Don't try to mutate state
  from inside a goroutine; return a `tea.Msg` instead.
- **Charmbracelet's bubbletea import path can be either**:
  - `github.com/charmbracelet/bubbletea` (long-stable v0/v1)
  - `charm.land/bubbletea/v2` (newer)
  
  Project uses **v2** per `go.mod`. Don't mix the two.
- **Terminal width**: degrade gracefully under 80 cols. Truncate symbol with
  ellipsis rather than wrap.
