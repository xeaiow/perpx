// Package confirm 提供 close-position 確認 modal 的 view 函式。
//
// 純 view layer — state（target/in-flight/result）由 positions tab 持有。
package confirm

import (
	"fmt"

	"github.com/yourname/poscli/internal/exchange"
	"github.com/yourname/poscli/internal/ui/numfmt"
	"github.com/yourname/poscli/internal/ui/styles"
)

// Render 把 modal 內容渲染為單一字串。
// inFlight=true 時顯示 "Submitting..."；否則顯示 y/n 提示。
func Render(p exchange.Position, inFlight bool) string {
	body := fmt.Sprintf(
		"Exchange:  %s\nSymbol:    %s\nSide:      %s\nSize:      %s\nMark:      %s\nEst. PnL:  %s USDT\n\nThis will submit a MARKET order to close the\nposition. This action is irreversible.\n\n",
		p.Exchange, p.Symbol, string(p.Side),
		numfmt.F(p.Size), numfmt.F(p.MarkPrice), numfmt.F(p.UnrealizedPnL),
	)
	if inFlight {
		body += styles.Dim.Render("Submitting...")
	} else {
		body += "Close this position?\n\n[y] yes    [n / esc] cancel"
	}
	return styles.Modal.Render(body)
}
