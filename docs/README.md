# docs/ index

These are detailed references. `CLAUDE.md` at the repo root is the
high-density session primer; each file here is a deeper dive for one topic.

**Load on-demand**, not all at once.

## When to read what

| File | When to read |
|---|---|
| `00-security.md` | Before any change to `internal/config/`. Before reviewing decryption error handling. |
| `01-exchange-interface.md` | Before implementing or modifying any exchange adapter. Read once, reference often. |
| `02-bybit.md` | Implementing the Bybit adapter (M2). |
| `03-binance.md` | Implementing the Binance adapter (part of M4). |
| `03-okx.md` | Implementing the OKX adapter (part of M4). |
| `03-bitget.md` | Implementing the Bitget adapter (part of M4). |
| `03-gate.md` | Implementing the Gate adapter (part of M4). Watch for SHA-512 + seconds timestamp. |
| `03-zoomex.md` | Implementing the Zoomex adapter (M3). Read 02-bybit.md first. |
| `04-tui.md` | Implementing or modifying the TUI (M5–M8). |

## Cross-cutting reading order

If a new contributor / new session needs full context, read in this order:

1. `CLAUDE.md` (root) — orient
2. `00-security.md` — why the crypto looks the way it does
3. `01-exchange-interface.md` — the contract everything ties to
4. Whichever `02-*` / `03-*` matches the current task
5. `04-tui.md` only if doing UI

## Conventions in these docs

- **Tables for endpoint references.** Easy to scan, hard to misread.
- **Code blocks are illustrative**, not always literal. Adapt to the project's
  helpers (e.g., `parseFloat`, `parseInt` in `internal/exchange/types.go`).
- **"Pitfalls collected"** at the bottom of each adapter doc. These are real
  mistakes from research or implementation. Add new ones when you hit them.
- **Testing checklists** are minimum bars, not full coverage. Add tests
  liberally; remove from the checklist only after they're written.
