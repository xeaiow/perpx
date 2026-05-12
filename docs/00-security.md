# 00 — Security model

This document is the canonical reference for everything secret-management.
Read this before touching `internal/config/crypto.go` or `internal/config/config.go`.

## Threat model

**Protected**: API keys, secrets, and passphrases for OKX/Bitget. The master password
itself is never persisted.

**In scope**:
- Disk theft (laptop loss, backup leak, accidental `git add`)
- Other local users on a shared host reading your home directory
- Casual shoulder-surfing of the config file

**Out of scope** (cannot be defended at this layer):
- Root on the same machine
- Memory dump / `ptrace` of the running process
- Malicious shell aliases or shim binaries replacing `poscli`
- A keylogger on the user's machine

## Crypto stack

```
master password (UTF-8 bytes)
    │
    ▼
Argon2id(time=3, memory=64MiB, parallelism=4, saltLen=16, keyLen=32)
    │
    ▼
32-byte AES key  ────►  AES-256-GCM
                            │
                            ▼
                  per-field random 12-byte nonce
                            │
                            ▼
                  Seal(nonce, plaintext, aad=nil)
                            │
                            ▼
            output bytes:  nonce(12) || ciphertext(N) || tag(16)
                            │
                            ▼
            "enc:" + base64.StdEncoding(...)
```

| Parameter | Value | Why |
|---|---|---|
| KDF | Argon2id | OWASP-recommended; memory-hard; resists GPU/ASIC |
| Iterations (t) | 3 | OWASP "interactive" baseline |
| Memory (m) | 64 MiB | OWASP "interactive" baseline; ~0.5s on commodity laptops |
| Parallelism (p) | 4 | Modern multi-core friendly |
| Salt length | 16 bytes | RFC 9106 §4 recommends ≥ 16 |
| Key length | 32 bytes | AES-256 |
| Cipher | AES-256-GCM | Authenticated; stdlib; constant-time on AES-NI |
| Nonce length | 12 bytes | GCM standard; never reuse with same key |

### Why these choices

- **Argon2id over scrypt/bcrypt**: hybrid resistance against both GPU and
  side-channel attacks. Pure Argon2d is GPU-resistant but vulnerable to side
  channels; Argon2i is the inverse. Argon2id covers both.
- **AES-GCM over ChaCha20-Poly1305**: stdlib has both; AES-GCM is constant-time
  on hardware with AES-NI (virtually all x86/ARM since 2011), and easier to
  reason about for auditors familiar with NIST primitives.
- **Random per-field nonce, not counter-based**: GCM nonce reuse with the same
  key is catastrophic (key recovery, not just confidentiality loss). 12 random
  bytes gives 2^48 safety margin before birthday collision is a concern; we
  encrypt ~10 fields per config, so this is comfortable.
- **Per-config salt, not per-field**: same password on different machines
  produces different keys. Per-field salt would inflate config size with no
  marginal security benefit since KDF is already slow.

## File format

```toml
[security]
salt = "base64-of-16-bytes"
kdf = "argon2id"

[security.kdf_params]
time = 3
memory = 65536
parallelism = 4

[runtime]
use_testnet = false
http_timeout_sec = 10

[exchanges.binance]
enabled = true
api_key = "enc:base64(nonce||ct||tag)"
api_secret = "enc:base64(nonce||ct||tag)"
# passphrase omitted — Binance does not use one

[exchanges.okx]
enabled = true
api_key = "enc:..."
api_secret = "enc:..."
passphrase = "enc:..."        # required for OKX and Bitget only
```

The `kdf_params` are stored so a future version can raise parameters without
breaking old configs (read params from file, derive with those, re-encrypt with
new params on `rotate-password`).

## Failure semantics

All decryption failures map to `ErrWrongPassword`. Concretely:

- Wrong master password → GCM tag mismatch → `ErrWrongPassword`
- Tampered ciphertext → GCM tag mismatch → `ErrWrongPassword`
- Truncated base64 → length check fails → `ErrWrongPassword`
- Invalid base64 → decode fails → `ErrWrongPassword`

This is **deliberate**. We do not tell the user "the password is correct but
the file is corrupt". Reasons:

1. We cannot actually distinguish those cases without the password.
2. Even if we could, exposing the distinction enables an attacker who can
   modify the config to learn whether they guessed the password (oracle attack).
3. Timing differences between paths would leak the same information; uniform
   handling removes that risk.

Internal/debug logs may distinguish for the developer; user-visible errors must not.

## Memory hygiene

Go provides no first-class secure-erase. We do the achievable subset:

- **Hold secrets as `[]byte`, not `string`.** Strings are immutable and may live
  in the runtime's string interning pool indefinitely.
- **Call `config.Zeroize(b)` before returning or on defer.** Overwrites the
  backing array with zeros.
- **Never `fmt.Print`, `log.Print`, or stringify secrets.**
- **Decrypt-then-pass-by-reference.** Pass `[]byte` into signers; do not
  copy into intermediate strings.

Acknowledged limits:
- GC may have already copied the underlying array (escape analysis dependent)
- A memory dump captures whatever's still live at the moment of dump
- The kernel may swap pages to disk (mlock would help; we don't use it because
  it requires CAP_IPC_LOCK or root on Linux)

These are acceptable losses given the threat model excludes root-level adversaries.

## File permissions

`Save` uses `OpenFile(path, O_WRONLY|O_CREATE|O_TRUNC, 0600)` plus an explicit
`Chmod(0600)` (umask defenses).

`Load` calls `checkFilePermissions(path)`:
- On Windows: skipped. NTFS ACLs are a different model; check would be wrong.
- Elsewhere: any bit set in `mode & 0o077` is fatal. Error message includes
  the offending mode and the `chmod 600` remediation command.

Do not relax this. A user who sets `chmod 644 config.toml` then leaks the file
on a multi-user box has effectively no defense remaining; we refuse loudly.

## Test coverage

`internal/config/crypto_test.go` and `config_test.go` together cover:

- Round-trip across empty, ASCII, Unicode, and 4 KiB payloads
- Wrong password returns `ErrWrongPassword`
- One-byte tampering returns `ErrWrongPassword`
- Non-encrypted field (missing `enc:` prefix) returns a *different* error
- Same plaintext + same key produces different ciphertext (nonce uniqueness)
- Wrong-length key rejected at Encrypt
- Zeroize actually zeros the buffer
- 0644 config refused with `ErrInsecurePermissions`
- OKX/Bitget without passphrase rejected
- `Save` writes 0600 even if umask is 0022

When modifying this layer:

```sh
go test -race -count=10 ./internal/config/
```

`-count=10` catches flaky randomness; `-race` catches concurrent misuse if
anyone later adds parallelism.
