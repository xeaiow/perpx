package config

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/BurntSushi/toml"
)

// ExchangeName 列舉支援的交易所識別字串。
type ExchangeName string

const (
	Binance ExchangeName = "binance"
	OKX     ExchangeName = "okx"
	Bybit   ExchangeName = "bybit"
	Bitget  ExchangeName = "bitget"
	Gate    ExchangeName = "gate"
	Zoomex  ExchangeName = "zoomex"
)

// AllExchanges 是支援的交易所固定清單，用於 init/add 命令的迴圈與驗證。
var AllExchanges = []ExchangeName{Binance, OKX, Bybit, Bitget, Gate, Zoomex}

// Security 是加密相關的全域設定，放在 [security] 段。
type Security struct {
	Salt      string    `toml:"salt"` // base64 encoded
	KDF       string    `toml:"kdf"`  // 目前固定 "argon2id"
	KDFParams KDFParams `toml:"kdf_params"`
}

// Runtime 是執行時偏好設定（非機密），放在 [runtime] 段。
type Runtime struct {
	UseTestnet     bool   `toml:"use_testnet"`
	HTTPTimeoutSec int    `toml:"http_timeout_sec"`
	LogFile        string `toml:"log_file"` // close-position log 寫入位置；空字串使用預設
}

func (r Runtime) defaults() Runtime {
	if r.HTTPTimeoutSec <= 0 {
		r.HTTPTimeoutSec = 10
	}
	return r
}

// ExchangeConfig 是單一交易所的 credential。
// 有些交易所（OKX、Bitget）需要 passphrase；其他人留空。
type ExchangeConfig struct {
	Enabled    bool   `toml:"enabled"`
	APIKey     string `toml:"api_key"`    // "enc:..."
	APISecret  string `toml:"api_secret"` // "enc:..."
	Passphrase string `toml:"passphrase"` // "enc:..." or "" if N/A
}

// Config 對應整個 config.toml。
//
// Exchanges map 的 key 是 string 而非 ExchangeName，因為 BurntSushi/toml
// encoder 不支援自訂 string 子型別當 map key。內部存取時再 cast。
type Config struct {
	Security  Security                    `toml:"security"`
	Runtime   Runtime                     `toml:"runtime"`
	Exchanges map[string]*ExchangeConfig  `toml:"exchanges"`
}

// Credentials 是解密後的單一交易所 credential，記憶體中使用。
type Credentials struct {
	Name       ExchangeName
	APIKey     []byte
	APISecret  []byte
	Passphrase []byte // 若該所不需要則為 nil
}

// Zeroize 清空所有敏感欄位。defer 呼叫。
func (c *Credentials) Zeroize() {
	if c == nil {
		return
	}
	Zeroize(c.APIKey)
	Zeroize(c.APISecret)
	Zeroize(c.Passphrase)
}

// LoadResult 是 Load 的回傳，把整個 config 跟解密好的 credentials 一起送出。
type LoadResult struct {
	Config      *Config
	Credentials map[ExchangeName]*Credentials // 只含 enabled=true 的交易所
}

// Zeroize 清掉所有 credentials。
func (r *LoadResult) Zeroize() {
	if r == nil {
		return
	}
	for _, c := range r.Credentials {
		c.Zeroize()
	}
}

// Load 讀取 config.toml、檢查權限、用 password 解密所有 enabled 的 credential。
//
// 解密任何一個欄位失敗都回 ErrWrongPassword（不洩漏細節）。
//
// 呼叫者必須在用完後呼叫 result.Zeroize()。
func Load(path string, password []byte) (*LoadResult, error) {
	if err := checkFilePermissions(path); err != nil {
		return nil, err
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.Runtime = cfg.Runtime.defaults()

	if cfg.Security.KDF != "argon2id" {
		return nil, fmt.Errorf("unsupported KDF %q (only argon2id is supported)", cfg.Security.KDF)
	}
	salt, err := decodeSaltB64(cfg.Security.Salt)
	if err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}

	key := DeriveKey(password, salt, cfg.Security.KDFParams)
	defer Zeroize(key)

	creds := make(map[ExchangeName]*Credentials)
	for nameStr, ec := range cfg.Exchanges {
		if !ec.Enabled {
			continue
		}
		name := ExchangeName(nameStr)
		c, err := decryptExchangeCreds(key, name, ec)
		if err != nil {
			// 清掉已經解到一半的，避免洩漏
			for _, decoded := range creds {
				decoded.Zeroize()
			}
			return nil, err
		}
		creds[name] = c
	}

	return &LoadResult{Config: &cfg, Credentials: creds}, nil
}

// Save 把 Config 寫回 path，並設定權限為 0600。
//
// 注意：此函式不做加密，呼叫者要先用 Encrypt 處理欄位。
// 通常 init/add/rotate 命令會手動建構 Config，把每個 credential 用 Encrypt 包好再寫入。
func Save(path string, cfg *Config) error {
	// 用 O_TRUNC | 0600 確保權限正確
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open config for write: %w", err)
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	enc.Indent = "  "
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}
	// 再次強制權限，以防 umask 影響
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("chmod 0600: %w", err)
	}
	return nil
}

// ErrInsecurePermissions 表示 config 檔的權限太寬鬆。
var ErrInsecurePermissions = errors.New("config file has insecure permissions")

// checkFilePermissions 檢查 config 檔權限不可比 0600 寬。
// Windows 跳過（NTFS ACL 模型不同）。
func checkFilePermissions(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat config: %w", err)
	}
	mode := info.Mode().Perm()
	// 只允許 owner read/write，其他位元都必須為 0
	if mode&0o077 != 0 {
		return fmt.Errorf("%w: %s has mode %#o, expected 0600. Run: chmod 600 %s",
			ErrInsecurePermissions, path, mode, path)
	}
	return nil
}

// requiresPassphrase 回傳該交易所是否需要 passphrase 欄位。
func requiresPassphrase(name ExchangeName) bool {
	switch name {
	case OKX, Bitget:
		return true
	default:
		return false
	}
}

// decryptExchangeCreds 解密單一交易所的 credential。
func decryptExchangeCreds(key []byte, name ExchangeName, ec *ExchangeConfig) (*Credentials, error) {
	apiKey, err := Decrypt(key, ec.APIKey)
	if err != nil {
		return nil, err
	}
	apiSecret, err := Decrypt(key, ec.APISecret)
	if err != nil {
		Zeroize(apiKey)
		return nil, err
	}

	c := &Credentials{
		Name:      name,
		APIKey:    apiKey,
		APISecret: apiSecret,
	}

	if requiresPassphrase(name) {
		if ec.Passphrase == "" {
			Zeroize(apiKey)
			Zeroize(apiSecret)
			return nil, fmt.Errorf("%s requires passphrase but field is empty", name)
		}
		pp, err := Decrypt(key, ec.Passphrase)
		if err != nil {
			Zeroize(apiKey)
			Zeroize(apiSecret)
			return nil, err
		}
		c.Passphrase = pp
	}
	return c, nil
}
