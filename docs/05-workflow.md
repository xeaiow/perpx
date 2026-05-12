# 05 — Milestone workflow

How to execute each remaining milestone. Each section is self-contained:
prerequisites, files to create, acceptance criteria, completion check.

After finishing a milestone:
1. Run `go build ./... && go test -race ./... && go vet ./...`
2. Update the status table in `CLAUDE.md`
3. Append a short note to this file's "Notes from completed work" section

## M2 — Bybit adapter

**Prereq**: read `docs/01-exchange-interface.md` and `docs/02-bybit.md`.

### Steps

1. Extend `internal/exchange/exchange.go` — add `AvailableBalance` to the interface.
   Update the file's interface doc to mention USDT only.
2. Create `internal/exchange/types.go` — house `parseFloat`, `parseInt`, normalization helpers.
3. Create `internal/exchange/errors.go` — sentinel errors `ErrAuth`, `ErrRateLimit`, etc.
4. Create `internal/exchange/registry.go` — `NewRegistry(*config.LoadResult)`.
5. Create `internal/exchange/bybit/`:
   - `client.go` — `Client`, `New`, `NewWithBaseURL`, `do()`, time sync
   - `positions.go`
   - `balance.go`
   - `history.go`
   - `close.go`
   - `types.go` — raw response structs
   - tests for each
6. Wire up `cmd/poscli/main.go`:
   - `run` subcommand: load config, prompt password, build registry, start TUI.
   - For M2 verification (no TUI yet), have `run` just print position counts:
     `binance: 3 positions, bybit: 1 position, ...` then exit.

### Acceptance

- [ ] All `go test ./...` pass
- [ ] `go vet ./...` clean
- [ ] `go test -race ./...` clean
- [ ] `poscli verify` still works (regression)
- [ ] With testnet credentials in `INTEGRATION_BYBIT=1` env, integration test
      hits the actual Bybit testnet and parses a real response
- [ ] Signing fixture test uses a fixed timestamp/key/secret with hardcoded
      expected signature; future changes that break signing will fail loudly

### Common mistakes to avoid

- Forgetting to filter `size==0` positions in `positions.go`
- Forgetting to flip side for closing fills in `history.go`
- Mutating the same `[]byte` slice for sign payload across goroutines (rare,
  but `do()` should be safe to call concurrently)

---

## M3 — Zoomex adapter

**Prereq**: M2 must be done. Read `docs/03-zoomex.md`.

### Steps

1. In `internal/exchange/bybit/client.go`, ensure `NewWithBaseURL` accepts
   `pathPrefix` argument and `do()` prepends it to every path.
2. Refactor Bybit's existing methods to call `do(ctx, "GET", "/position/list", ...)`
   (relative path), not `/v5/position/list`.
3. Add path-overridable variants where Zoomex differs:
   - `HistoryAtPath(ctx, path, since)` for the `closed-pnl` vs `close-pnl` divergence
4. Create `internal/exchange/zoomex/client.go` — thin wrapper using composition.
5. Register in `internal/exchange/registry.go`.

### Acceptance

- [ ] Zoomex client constructed correctly for mainnet and testnet URLs
- [ ] Position results have `Exchange == "zoomex"`
- [ ] Bybit tests still pass after refactoring (no regression)
- [ ] Integration test with `INTEGRATION_ZOOMEX=1` confirms real testnet works

---

## M4 — Binance, OKX, Bitget, Gate

Four adapters in parallel. Each is a duplicate of the Bybit pattern but with
exchange-specific signing, paths, and quirks. Read the matching `03-*.md`.

### Per-exchange checklist

For each of binance/okx/bitget/gate:

1. Read `docs/03-<name>.md` end to end before writing code.
2. Create `internal/exchange/<name>/` with the same five files (client, positions,
   balance, history, close + tests).
3. Pay attention to the **pitfalls** section; those are pre-known footguns.
4. Register in `registry.go`.

### Order suggestion (easiest to hardest)

1. **OKX** — has dedicated close-position and history endpoints. Signing similar to Bybit.
2. **Bitget** — also has dedicated endpoints. Five-zero success code is the only surprise.
3. **Binance** — no native history endpoint, must aggregate from income. Form-encoded POST.
4. **Gate** — SHA-512, seconds timestamp, contracts-not-coin size. Most distinctive.

### Acceptance for M4 as a whole

- [ ] All four adapters implement the interface
- [ ] All registered in `registry.go`
- [ ] All have signing unit tests with fixtures
- [ ] All have HTTP roundtrip tests covering happy + auth + rate-limit paths
- [ ] `poscli run` prints counts from all five enabled adapters in parallel

---

## M5 — Positions tab

**Prereq**: M2 done (so there's at least one working adapter to display). Read `docs/04-tui.md`.

### Steps

1. Create `internal/ui/styles/styles.go` — colors, table styles, PnL coloring.
2. Create `internal/ui/messages.go` — shared msg types.
3. Create `internal/ui/positions/`:
   - `model.go` — Model with `bubbles/table`, exchanges map, lastFetch
   - `fetch.go` — parallel fetch tea.Cmd
   - `update.go` — handle KeyMsg, FetchedMsg, tea.WindowSizeMsg
   - `view.go` — render table + status line
4. Create `internal/ui/app.go` — root model that owns tab state and routes msgs.
5. Wire `cmd/poscli/main.go`'s `run` subcommand to call `tea.NewProgram(ui.New(registry))`.

### Acceptance

- [ ] `poscli run` starts the TUI
- [ ] Positions tab fetches from all enabled exchanges in parallel on startup
- [ ] Selection moves up/down with `j/k` and arrow keys
- [ ] `r` triggers refresh
- [ ] `q` quits cleanly (restores terminal)
- [ ] Per-exchange errors shown in status area; other exchanges still display
- [ ] Negative uPnL renders in red, positive in green

---

## M6 — Close-position flow

### Steps

1. Create `internal/ui/confirm/modal.go` — confirmation overlay.
2. Wire `x` key in positions tab → set modal state with selected `Position`.
3. Modal `y` handler → `CloseCmd(exchange, request)` → on result, return
   `CloseResultMsg`, close modal, trigger refresh.
4. Status toast for close result (success / error).

### Acceptance

- [ ] Pressing `x` on a selected row opens modal
- [ ] Modal shows exchange / symbol / side / size / mark / est PnL
- [ ] `y` submits and shows progress
- [ ] `n` or `esc` cancels
- [ ] Successful close triggers refresh, position disappears from table
- [ ] Failed close shows error toast for ≥3 seconds
- [ ] No way to dismiss modal other than `y` / `n` / `esc`

---

## M7 — History tab

### Steps

1. Create `internal/ui/history/` with same model+update+view+fetch shape.
2. Use `bubbles/viewport` if list is long; otherwise reuse table.
3. Exchange filter cycle on `f` key.
4. Default `since = now() - 7d`; let exchanges adjust as their API allows.

### Acceptance

- [ ] History fetches from all enabled exchanges
- [ ] Combined list sorted by close time descending
- [ ] `f` cycles through exchange filters
- [ ] Binance's degraded data (no entry/exit price) renders as `—`, not crash
- [ ] Empty result shows "No history in the last 7 days"

---

## M8 — Accounts tab + polish

### Steps

1. Create `internal/ui/accounts/` with model+update+view+fetch.
2. Equity per exchange = `AvailableBalance + Σ position.Notional`. The
   positions are already cached from Positions tab fetches; can either share
   state or refetch.
3. Render single-column "Equity (USDT)" table with totals.
4. Help overlay (`?` key) listing all key bindings.
5. README update: screenshot, install instructions, security notes.

### Acceptance

- [ ] Accounts tab shows equity per enabled exchange + total
- [ ] Per-exchange failure shown as `—` with footnote
- [ ] `?` shows full key-binding reference
- [ ] README installs cleanly via `go install`

---

## Notes from completed work

### M0 (skeleton)
- Cobra command structure. `init`, `verify` fully implemented; `add`, `rotate-password`, `run` stubs.
- Discovered BurntSushi/toml cannot encode `map[CustomStringType]V`. Use `map[string]V` and cast.

### M1 (secret management)
- Argon2id parameters: t=3, m=64MiB, p=4. ~0.5s on commodity laptops.
- Per-config salt, per-field nonce.
- All decryption failures uniformly return `ErrWrongPassword`.
- 0600 permissions enforced on Unix; Windows skipped (different ACL model).
- 13 unit tests passing.

### M2 (Bybit adapter)
- `internal/exchange/{types.go, errors.go, registry.go}` 建立。
- Bybit V5：HMAC-SHA256 on `ts + apiKey + recvWindow + (query OR body)`；time sync via `/v5/market/time`；retCode 10002 → resync + retry once。
- `NewWithBaseURL(creds, rt, baseURL, pathPrefix)` 與 `SetName(name)` 預埋給 M3 Zoomex 用。
- 簽章 fixture 預期值用 Python `hmac.new(secret, payload, hashlib.sha256).hexdigest()` 獨立驗算。

### M3 (Zoomex)
- Composition 包 `bybit.Client`：`baseURL=https://openapi.zoomex.com`、`pathPrefix=/cloud/trade/v3`、`SetName("zoomex")`。
- History 路徑為 `/position/close-pnl`（非 `closed-pnl`），透過 `HistoryAtPath` 覆寫。

### M4 (Binance / OKX / Bitget / Gate)
- 各家簽章在 client.go 中實作；fixture 預期值都用 Python 獨立算過避免「以實作回填預期」。
- Binance：form-encoded POST、`positionAmt` 帶正負號。
- OKX：ISO 8601 timestamp + base64(HMAC-SHA256)、`code` 是字串 `"0"`；passphrase 為 header。
- Bitget：簽章同 OKX，但 timestamp 是 unix ms 字串、成功碼是 `"00000"`（五個零）。
- Gate：HMAC-SHA512、Unix 秒、size 是 contracts 需乘 `quanto_multiplier`、close 用 `size=0+close=true` idiom。

### M5–M8 (TUI)
- 用 Bubble Tea v2：`charm.land/bubbletea/v2`、`charm.land/bubbles/v2`、`charm.land/lipgloss/v2`。
- 跨套件 msg 型別放在 `internal/ui/uimsg`，避免 `internal/ui` 與 sub-tabs 的 import cycle。
- v2 介面與 v1 差異：`Init() Cmd`、`Update(Msg) (Model, Cmd)`、`View() View`；alt screen 用 `view.AltScreen = true` 設定。
- positions tab 即時平行抓所有 adapter；close-position confirmation modal 由 root app 接管 `x/y/n/esc` 鍵；CloseResultMsg 觸發 toast + refresh。
- history tab 由 close time 反向排序；`f` cycle filter；空資料顯示 placeholder。
- accounts tab equity = available + Σ position notional；失敗顯示 `—`。
- help overlay 由 `?` toggle。
