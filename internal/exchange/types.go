package exchange

import (
	"strconv"
	"strings"
)

// ParseFloat 解析 API 回傳的字串數字。空字串視為 0、無錯誤。
// 非空字串必須能 clean parse，否則回錯。
func ParseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// MustParseFloat 跟 ParseFloat 一樣，但失敗時回 0；給 adapter 內已知格式時用。
// adapter 自行決定要不要寬鬆處理。
func MustParseFloat(s string) float64 {
	v, _ := ParseFloat(s)
	return v
}

// ParseInt 解析 API 回傳的整數字串（多半是 unix ms）。空字串視為 0。
func ParseInt(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

// MustParseInt 跟 ParseInt 一樣，但失敗時回 0。
func MustParseInt(s string) int64 {
	v, _ := ParseInt(s)
	return v
}
