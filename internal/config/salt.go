package config

import "encoding/base64"

// encodeSaltB64 把 salt bytes 編成 base64 字串以放進 toml。
func encodeSaltB64(salt []byte) string {
	return base64.StdEncoding.EncodeToString(salt)
}

// decodeSaltB64 把 toml 中的字串還原成 salt bytes。
func decodeSaltB64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
