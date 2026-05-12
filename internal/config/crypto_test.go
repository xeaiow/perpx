package config

import (
	"bytes"
	"errors"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	salt, err := NewSalt()
	if err != nil {
		t.Fatalf("NewSalt: %v", err)
	}
	key := DeriveKey([]byte("correct horse battery staple"), salt, DefaultKDFParams())
	defer Zeroize(key)

	cases := []string{
		"",
		"a",
		"sk-abc123-some-api-key",
		"極長的金鑰字串測試 unicode 內容 🔐 with mixed bytes",
		string(bytes.Repeat([]byte("x"), 4096)),
	}
	for _, plain := range cases {
		enc, err := Encrypt(key, []byte(plain))
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", plain, err)
		}
		if !IsEncrypted(enc) {
			t.Fatalf("Encrypt output missing prefix: %q", enc)
		}
		got, err := Decrypt(key, enc)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if string(got) != plain {
			t.Errorf("round-trip mismatch:\n got: %q\nwant: %q", got, plain)
		}
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	salt, _ := NewSalt()
	rightKey := DeriveKey([]byte("right"), salt, DefaultKDFParams())
	wrongKey := DeriveKey([]byte("wrong"), salt, DefaultKDFParams())
	defer Zeroize(rightKey)
	defer Zeroize(wrongKey)

	enc, _ := Encrypt(rightKey, []byte("secret"))
	_, err := Decrypt(wrongKey, enc)
	if !errors.Is(err, ErrWrongPassword) {
		t.Errorf("expected ErrWrongPassword, got %v", err)
	}
}

func TestDecryptTampered(t *testing.T) {
	salt, _ := NewSalt()
	key := DeriveKey([]byte("pw"), salt, DefaultKDFParams())
	defer Zeroize(key)

	enc, _ := Encrypt(key, []byte("secret"))
	// 翻轉最後一個 base64 字元，破壞 auth tag
	tampered := enc[:len(enc)-1] + flipBase64Char(enc[len(enc)-1])
	_, err := Decrypt(key, tampered)
	if !errors.Is(err, ErrWrongPassword) {
		t.Errorf("tampered ciphertext should fail: got %v", err)
	}
}

func TestDecryptNotEncrypted(t *testing.T) {
	key := make([]byte, keyLen)
	_, err := Decrypt(key, "plaintext-without-prefix")
	if err == nil {
		t.Error("expected error for non-encrypted field")
	}
	if errors.Is(err, ErrWrongPassword) {
		t.Error("should distinguish 'not encrypted' from 'wrong password'")
	}
}

func TestNonceUniqueness(t *testing.T) {
	// GCM 重用 nonce = 災難。確認同樣 plaintext + 同樣 key 加密兩次產生不同 ciphertext。
	salt, _ := NewSalt()
	key := DeriveKey([]byte("pw"), salt, DefaultKDFParams())
	defer Zeroize(key)

	enc1, _ := Encrypt(key, []byte("same"))
	enc2, _ := Encrypt(key, []byte("same"))
	if enc1 == enc2 {
		t.Error("same plaintext encrypted twice produced identical output — nonce reused!")
	}
}

func TestKeyWrongLength(t *testing.T) {
	short := make([]byte, 16)
	_, err := Encrypt(short, []byte("x"))
	if err == nil {
		t.Error("expected error for wrong key length")
	}
}

func TestZeroize(t *testing.T) {
	b := []byte("sensitive")
	Zeroize(b)
	for i, v := range b {
		if v != 0 {
			t.Errorf("byte %d not zero: %d", i, v)
		}
	}
}

func flipBase64Char(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}
