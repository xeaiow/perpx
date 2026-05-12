package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// helper: 建一個包含兩家交易所的測試 config，回傳 path + password
func mkTestConfig(t *testing.T, dir string) (string, []byte) {
	t.Helper()
	password := []byte("test-password-123")

	salt, err := NewSalt()
	if err != nil {
		t.Fatalf("NewSalt: %v", err)
	}
	kdf := DefaultKDFParams()
	key := DeriveKey(password, salt, kdf)
	defer Zeroize(key)

	binKey, _ := Encrypt(key, []byte("binance-api-key"))
	binSec, _ := Encrypt(key, []byte("binance-api-secret"))
	okxKey, _ := Encrypt(key, []byte("okx-api-key"))
	okxSec, _ := Encrypt(key, []byte("okx-api-secret"))
	okxPP, _ := Encrypt(key, []byte("okx-passphrase"))

	cfg := &Config{
		Security: Security{
			Salt:      encodeSaltB64(salt),
			KDF:       "argon2id",
			KDFParams: kdf,
		},
		Runtime: Runtime{UseTestnet: true, HTTPTimeoutSec: 5},
		Exchanges: map[string]*ExchangeConfig{
			string(Binance): {Enabled: true, APIKey: binKey, APISecret: binSec},
			string(OKX):     {Enabled: true, APIKey: okxKey, APISecret: okxSec, Passphrase: okxPP},
			// Bybit enabled=false，不會被解密
			string(Bybit): {Enabled: false, APIKey: "enc:invalid", APISecret: "enc:invalid"},
		},
	}

	path := filepath.Join(dir, "config.toml")
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return path, password
}

func TestLoadHappyPath(t *testing.T) {
	dir := t.TempDir()
	path, password := mkTestConfig(t, dir)

	result, err := Load(path, password)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defer result.Zeroize()

	if len(result.Credentials) != 2 {
		t.Fatalf("expected 2 enabled creds, got %d", len(result.Credentials))
	}

	bin := result.Credentials[Binance]
	if bin == nil {
		t.Fatal("missing binance creds")
	}
	if string(bin.APIKey) != "binance-api-key" {
		t.Errorf("binance api key mismatch: %q", bin.APIKey)
	}
	if string(bin.APISecret) != "binance-api-secret" {
		t.Errorf("binance api secret mismatch")
	}
	if bin.Passphrase != nil {
		t.Errorf("binance shouldn't have passphrase")
	}

	okx := result.Credentials[OKX]
	if okx == nil {
		t.Fatal("missing okx creds")
	}
	if string(okx.Passphrase) != "okx-passphrase" {
		t.Errorf("okx passphrase mismatch: %q", okx.Passphrase)
	}

	if _, ok := result.Credentials[Bybit]; ok {
		t.Error("disabled exchange should not appear in creds map")
	}
}

func TestLoadWrongPassword(t *testing.T) {
	dir := t.TempDir()
	path, _ := mkTestConfig(t, dir)

	_, err := Load(path, []byte("wrong-password"))
	if !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}
}

func TestLoadInsecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check skipped on windows")
	}
	dir := t.TempDir()
	path, password := mkTestConfig(t, dir)

	if err := os.Chmod(path, 0644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	_, err := Load(path, password)
	if !errors.Is(err, ErrInsecurePermissions) {
		t.Fatalf("expected ErrInsecurePermissions, got %v", err)
	}
}

func TestLoadOKXMissingPassphrase(t *testing.T) {
	dir := t.TempDir()
	password := []byte("pw")
	salt, _ := NewSalt()
	key := DeriveKey(password, salt, DefaultKDFParams())
	defer Zeroize(key)

	okxKey, _ := Encrypt(key, []byte("k"))
	okxSec, _ := Encrypt(key, []byte("s"))

	cfg := &Config{
		Security: Security{Salt: encodeSaltB64(salt), KDF: "argon2id", KDFParams: DefaultKDFParams()},
		Exchanges: map[string]*ExchangeConfig{
			string(OKX): {Enabled: true, APIKey: okxKey, APISecret: okxSec /* missing Passphrase */},
		},
	}
	path := filepath.Join(dir, "config.toml")
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path, password)
	if err == nil {
		t.Fatal("expected error for missing okx passphrase")
	}
}

func TestSaveCreatesFileWith0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission check skipped on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{
		Security: Security{Salt: "AA==", KDF: "argon2id", KDFParams: DefaultKDFParams()},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("expected mode 0600, got %#o", mode)
	}
}

func TestRequiresPassphrase(t *testing.T) {
	cases := map[ExchangeName]bool{
		OKX:     true,
		Bitget:  true,
		Binance: false,
		Bybit:   false,
		Gate:    false,
		Zoomex:  false,
	}
	for name, want := range cases {
		if got := requiresPassphrase(name); got != want {
			t.Errorf("requiresPassphrase(%s) = %v, want %v", name, got, want)
		}
	}
}
