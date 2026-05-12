# CLAUDE.md

This file is read by Claude Code at the start of each session. Keep it concise,
action-oriented, and current. Detailed references live in `docs/`.

## What this project is

`poscli` is a Go TUI for managing **USDT-M perpetual futures positions** across
six exchanges: Binance, OKX, Bybit, Bitget, Gate, Zoomex.

Three tabs: **Positions** (current) · **History** (closed) · **Accounts** (equity per exchange).
Single-position close via market-reduce order, confirmed by `y/n` prompt.

API keys live in `config.toml`, encrypted with AES-256-GCM under an Argon2id key
derived from a master password entered at startup.

**Non-goals**: opening positions, modifying leverage, TP/SL, copy-trading.

## Tech stack (pinned)

- **Go**: 1.22+
- **TUI**: `charm.land/bubbletea/v2` + `github.com/charmbracelet/bubbles` + `lipgloss`
- **CLI**: `github.com/spf13/cobra`
- **TOML**: `github.com/BurntSushi/toml`
- **Crypto**: `golang.org/x/crypto/argon2` + stdlib `crypto/aes` + `crypto/cipher`
- **Terminal IO**: `golang.org/x/term`

Do not introduce new dependencies without explicit approval.

## Repository layout

```
cmd/poscli/                     CLI entry (cobra). main.go has init/add/verify/rotate/run.
internal/config/                Secret management. DONE.
  crypto.go                     Argon2id + AES-256-GCM. Encrypt/Decrypt/DeriveKey.
  config.go                     TOML load/save, permission check (0600), passphrase rules.
internal/exchange/              Adapter abstraction.
  exchange.go                   Exchange interface + Position/ClosedPosition/Credentials types.
  binance/  okx/  bybit/        Per-exchange adapters. Bybit first; Zoomex extends Bybit.
  bitget/   gate/  zoomex/
internal/ui/                    Bubble Tea. NOT YET STARTED.
  positions/  history/  accounts/  confirm/  styles/
docs/                           Detailed references. Load only when needed.
```

## Current status

| Milestone | Status | Notes |
|---|---|---|
| M0 — skeleton, cobra commands | done | `init`/`verify` work end-to-end |
| M1 — secret management | done | 13 tests passing; covers crypto + permission + missing passphrase |
| M2 — Bybit adapter | done | client/positions/balance/history/close + signing fixture + httptest |
| M3 — Zoomex (extends Bybit) | done | Composition; base URL + path prefix + close-pnl override |
| M4 — Binance/OKX/Bitget/Gate adapters | done | All four implement the interface; signing fixtures verified by Python HMAC |
| M5 — TUI Positions tab | done | Bubble Tea v2 (`charm.land/bubbletea/v2`); parallel fetch + table + status line |
| M6 — Close-position flow | done | `x` opens modal, `y/n/esc`, toast + refresh on result |
| M7 — History tab | done | Sort desc by close time; `f` cycles filter; empty placeholder |
| M8 — Accounts tab + polish | done | Equity per exchange; total row; help overlay (?); — for failed |

## Build & test commands

```sh
go build ./...                              # compile everything
go build -o /tmp/poscli ./cmd/poscli        # build binary
go test ./...                               # all tests
go test -v ./internal/config/               # verbose, single package
go test -race ./...                         # race detector (run before commits)
go vet ./...                                # static check
```

After every change in this repo:
1. `go build ./...` — must succeed
2. `go test ./...` — must pass
3. `go vet ./...` — must be silent

## Coding conventions

- **Doc comments in English, code comments may be Chinese.** User-facing strings and
  errors that surface to the user can be Chinese; library-level identifiers, package
  docs, and exported godoc must be English.
- **Errors**: wrap with `fmt.Errorf("context: %w", err)`. Return sentinel errors
  (`var ErrXxx = errors.New(...)`) when callers need to branch on them.
- **Crypto secrets**: hold as `[]byte`, never `string`. Defer `config.Zeroize(b)` on
  any decrypted secret. Never log them.
- **HTTP clients**: always pass `context.Context`. Honor cancellation. Default
  timeout from `cfg.Runtime.HTTPTimeoutSec`.
- **No `panic`** in library code. CLI entry can `os.Exit(1)` on fatal startup errors.
- **TOML map keys** must be `string`, not custom string types. BurntSushi/toml
  panics on reflect-unassignable map keys. (Learned the hard way in M1.)
- **One file per concern**: `positions.go`, `history.go`, `close.go`, `balance.go`
  inside each adapter. `client.go` holds the signing logic and shared HTTP helper.

## Critical safety rules

These cannot be relaxed. If a task seems to require violating one, stop and ask.

1. **Never log, print, or include in error messages**: API keys, secrets, passphrases,
   master password, or derived keys. Tests must not assert on their contents either.
2. **`Decrypt` failures all return `ErrWrongPassword`.** Do not leak whether it was
   the password vs corrupted ciphertext vs invalid base64 — timing/side-channel risk.
3. **Close-position is irreversible.** The TUI must show a confirmation modal before
   submitting any `ClosePosition` request. `y` confirms, `n`/`esc` cancels.
4. **API key permission scope**: when documenting setup, instruct users to enable
   only Read + Trade. Never Transfer/Withdrawal.
5. **Config file permissions**: `Save` writes 0600; `Load` rejects anything looser
   (Unix only; Windows skipped). Do not weaken this check.
6. **No new network calls in `verify`.** It only decrypts; it must never call exchange APIs.

## How to work on this codebase

### Adding a new exchange adapter

1. Read `docs/01-exchange-interface.md` — the contract every adapter implements.
2. Read the per-exchange doc (`docs/02-bybit.md`, `docs/03-binance.md`, etc.) for
   endpoints, signing, and quirks.
3. Create `internal/exchange/<name>/`:
   - `client.go` — signing, shared `do()` helper
   - `positions.go`, `balance.go`, `history.go`, `close.go`
4. Register in `internal/exchange/registry.go` (create if not present).
5. Write tests:
   - Sign helper unit test (fixture from official docs)
   - `httptest.Server` based test for each endpoint's parsing + error paths
6. Update this CLAUDE.md status table.

### Modifying secret management

Almost never the right move. If you must, read `docs/00-security.md` first and
preserve all guarantees (auth tag, nonce randomness, password-error uniformity,
0600 permissions). Re-run the full `internal/config/` test suite with `-race`.

### Touching the TUI

`docs/04-tui.md` has the layout, key bindings, and message flow. Bubble Tea
follows Elm architecture: model→update→view. Don't put I/O in `View()`; use
`tea.Cmd` for async work.

## When in doubt

- Pull from `docs/` files for detail. Each is named for what it answers.
- Refuse to invent endpoint paths or response field names. If the doc doesn't
  cover it, ask before writing the call.
- If a task says "implement X" but the doc is missing or unclear, write the doc
  stub first and ask the user to fill it.
