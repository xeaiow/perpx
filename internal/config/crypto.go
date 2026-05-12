// Package config 處理 config.toml 的載入、加密欄位的解密，以及金鑰衍生。
//
// 加密設計：
//
//	master password ──Argon2id(salt)──▶ 32-byte key
//	plaintext ──AES-256-GCM(key, random nonce)──▶ ciphertext+tag
//	欄位儲存：base64( nonce || ciphertext || tag )，前綴 "enc:"
//
// 為什麼這樣選：
//   - Argon2id 抗 GPU/ASIC，OWASP 推薦的 password hashing
//   - AES-GCM 自帶 auth tag，可偵測篡改；GCM 不重用 nonce，每次加密重新隨機產生
//   - salt 全 config 一份（在 [security] 段），nonce 每個欄位一份
package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// 加密欄位前綴。未加密時直接是空字串或不含此前綴。
	encPrefix = "enc:"

	// AES-256-GCM 用 32-byte key。
	keyLen = 32

	// GCM 標準 nonce 長度 12 bytes；改成其他長度需要 NewGCMWithNonceSize，不建議。
	nonceLen = 12

	// Salt 長度 16 bytes 足夠（Argon2 RFC 推薦 ≥ 16）。
	saltLen = 16
)

// KDFParams 是 Argon2id 的調整參數。寫進 config 以便日後升級而不破壞舊檔。
type KDFParams struct {
	Time        uint32 `toml:"time"`        // iterations
	Memory      uint32 `toml:"memory"`      // KiB
	Parallelism uint8  `toml:"parallelism"` // threads
}

// DefaultKDFParams 採 OWASP 2023 對 Argon2id 的中等敏感建議：
//
//	time=3, memory=64MiB, parallelism=4
//
// 對 CLI 啟動延遲 ~0.5s 級數，可接受。
func DefaultKDFParams() KDFParams {
	return KDFParams{
		Time:        3,
		Memory:      64 * 1024, // 64 MiB
		Parallelism: 4,
	}
}

// ErrWrongPassword 表示解密失敗。我們不區分「密碼錯」「ciphertext 被改」「nonce 壞」，
// 都統一回這個錯，以免時序攻擊或誘導使用者猜密碼正確性。
var ErrWrongPassword = errors.New("decryption failed: wrong password or corrupted config")

// DeriveKey 從密碼 + salt 衍生 32-byte AES key。
// 呼叫者用完務必 Zeroize(key)。
func DeriveKey(password []byte, salt []byte, p KDFParams) []byte {
	return argon2.IDKey(password, salt, p.Time, p.Memory, p.Parallelism, keyLen)
}

// NewSalt 生成新的隨機 salt。init 時呼叫一次寫進 config。
func NewSalt() ([]byte, error) {
	s := make([]byte, saltLen)
	if _, err := rand.Read(s); err != nil {
		return nil, fmt.Errorf("rand salt: %w", err)
	}
	return s, nil
}

// Encrypt 用 key 加密 plaintext，回傳 "enc:" 前綴的 base64 字串。
// 每次呼叫都產生新的 random nonce。
func Encrypt(key, plaintext []byte) (string, error) {
	if len(key) != keyLen {
		return "", fmt.Errorf("encrypt: key must be %d bytes, got %d", keyLen, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("encrypt: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encrypt: new gcm: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("encrypt: rand nonce: %w", err)
	}

	// Seal 把 ciphertext 跟 tag 接在一起，回傳 nonce||ct||tag
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	buf := make([]byte, 0, len(nonce)+len(ct))
	buf = append(buf, nonce...)
	buf = append(buf, ct...)

	return encPrefix + base64.StdEncoding.EncodeToString(buf), nil
}

// Decrypt 解密 "enc:..." 字串，回傳 plaintext。
// 呼叫者用完務必 Zeroize 回傳的 []byte。
//
// 注意：任何錯誤（base64 壞、長度不對、auth tag 不符）一律回 ErrWrongPassword，
// 不洩漏細節避免時序攻擊。詳細錯誤只在 debug log 看得到。
func Decrypt(key []byte, field string) ([]byte, error) {
	if !IsEncrypted(field) {
		return nil, errors.New("decrypt: field is not encrypted (missing 'enc:' prefix)")
	}
	if len(key) != keyLen {
		return nil, fmt.Errorf("decrypt: key must be %d bytes, got %d", keyLen, len(key))
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(field, encPrefix))
	if err != nil {
		return nil, ErrWrongPassword
	}
	if len(raw) < nonceLen+16 { // 至少要有 nonce + 16-byte GCM tag
		return nil, ErrWrongPassword
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrWrongPassword
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrWrongPassword
	}

	nonce, ct := raw[:nonceLen], raw[nonceLen:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrWrongPassword
	}
	return pt, nil
}

// IsEncrypted 判斷欄位是否為加密格式。
func IsEncrypted(field string) bool {
	return strings.HasPrefix(field, encPrefix)
}

// Zeroize 把敏感 buffer 清零。Go 的 GC 不保證 string/byte 何時消失，
// 至少明確抹掉我們持有的副本。
//
// 用法：
//
//	pwd := readPassword()
//	defer Zeroize(pwd)
func Zeroize(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
