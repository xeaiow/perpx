# poscli

Multi-exchange perpetual position manager (TUI) for Binance, OKX, Bybit, Bitget, Gate, and Zoomex.

API keys are stored encrypted in `config.toml` (AES-256-GCM with Argon2id-derived key),
unlocked by a master password at startup.

## Status

| Milestone | Status |
|---|---|
| M0 — Project skeleton + CLI subcommands | ✅ Done |
| M1 — Secret management (Argon2id + AES-GCM) | ✅ Done |
| M2 — Bybit adapter | ✅ Done |
| M3 — Zoomex adapter (reuses Bybit) | ⏳ Next |
| M4 — Binance / OKX / Bitget / Gate adapters | |
| M5 — TUI Positions tab | |
| M6 — Close-position flow with confirmation | |
| M7 — History tab | |
| M8 — Accounts tab + polish | |

## Build

```sh
go build -o poscli ./cmd/poscli
```

Requires Go 1.22+.

## Quick start

```sh
# 1. Create config (interactive; prompts for master password + each exchange's keys)
./poscli init

# 2. Verify config can be decrypted
./poscli verify

# 3. Run TUI (not yet implemented — wait for M5)
./poscli run
```

The config defaults to `~/.config/poscli/config.toml`. Override with `--config <path>`.

## Security

- **Master password** → Argon2id (`time=3, memory=64MiB, parallelism=4`) → 32-byte AES key
- **Per-field encryption**: each `api_key` / `api_secret` / `passphrase` has its own random
  12-byte nonce. Same plaintext encrypted twice produces different ciphertext.
- **Auth tag**: AES-GCM tag detects any tampering of the config file.
- **File permissions**: forced to `0600`. Startup refuses to load configs with looser perms.
- **Memory hygiene**: decrypted secrets are held as `[]byte` and zeroed before exit.
- **API key permissions**: when creating keys at each exchange, enable only **Read + Trade**.
  Do not enable Transfer/Withdrawal — this tool only needs to view and close positions.

## Project layout

```
cmd/poscli/                 CLI entry (cobra)
internal/config/            TOML parsing, Argon2id+AES-GCM, permission checks
internal/exchange/          Exchange interface and common types
  binance/  okx/  bybit/    Per-exchange adapters (to be implemented)
  bitget/   gate/  zoomex/
internal/ui/                Bubble Tea TUI (to be implemented)
```

## Tests

```sh
go test ./...
```

The crypto layer has full coverage: round-trip, wrong-password detection, tampering
detection, nonce uniqueness, key-length validation, file permission enforcement, and
exchange-specific passphrase requirements.
