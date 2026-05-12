// Package numfmt 集中所有 TUI 內的數字顯示格式。
//
// 規則：最多 3 位小數，尾隨 0 不補。整數顯示為整數。
// 負數保留 "-"；正數不加 "+"。
package numfmt

import (
	"strconv"
	"strings"
)

// F 把 float64 轉為「最多 3 位小數、無補 0」的字串。
//
//	0.812345 → "0.812"
//	10.24    → "10.24"
//	542      → "542"
//	13.0     → "13"
//	-4.05    → "-4.05"
//	0        → "0"
func F(v float64) string {
	// strconv.FormatFloat with prec=3 then trim trailing zeros.
	// 注意：'f' 格式會固定 3 位、我們手動把尾 0 與小數點去掉。
	s := strconv.FormatFloat(v, 'f', 3, 64)
	if dot := strings.IndexByte(s, '.'); dot >= 0 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}
