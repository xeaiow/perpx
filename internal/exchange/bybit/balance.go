package bybit

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/yourname/poscli/internal/exchange"
)

// AvailableBalance 回傳 USDT 可動用餘額。
//
// 先試 UNIFIED；若回 accountType 不支援的特定錯誤碼，退回 CONTRACT。
func (c *Client) AvailableBalance(ctx context.Context) (float64, error) {
	bal, err := c.fetchBalance(ctx, "UNIFIED")
	if err != nil && errors.Is(err, errAccountTypeUnsupported) {
		return c.fetchBalance(ctx, "CONTRACT")
	}
	return bal, err
}

var errAccountTypeUnsupported = errors.New("bybit: account type unsupported")

func (c *Client) fetchBalance(ctx context.Context, accountType string) (float64, error) {
	q := url.Values{}
	q.Set("accountType", accountType)
	q.Set("coin", "USDT")

	var raw rawWalletBalance
	err := c.do(ctx, http.MethodGet, "/account/wallet-balance", q, nil, &raw)
	if err != nil {
		// Bybit 對非 UTA 帳戶呼叫 UNIFIED 時可能回 retCode 此處用 RetMsg 判斷不夠穩，
		// 用簡化策略：呼叫端只在 UNIFIED 出錯時 fallback；包成 errAccountTypeUnsupported。
		// 為簡單起見，這裡只在 retMsg 含 "accountType" 或 "account type" 的時候 fallback。
		if isAccountTypeErr(err) {
			return 0, errAccountTypeUnsupported
		}
		return 0, err
	}

	for _, acc := range raw.List {
		for _, coin := range acc.Coin {
			if coin.Coin != "USDT" {
				continue
			}
			// availableToWithdraw 可能是空字串（UTA 在某些情境下會回空）；
			// 退回 walletBalance。
			v, _ := exchange.ParseFloat(coin.AvailableToWithdraw)
			if v == 0 {
				v, _ = exchange.ParseFloat(coin.WalletBalance)
			}
			return v, nil
		}
	}
	return 0, nil
}

// isAccountTypeErr 粗略偵測「帳戶類型不支援」錯誤。Bybit 沒有專屬碼，
// 多半在 retMsg 裡帶 "account" 字眼。誤判時頂多多打一個 CONTRACT 請求。
func isAccountTypeErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, kw := range []string{"accountType", "account type", "AccountType"} {
		if containsFold(msg, kw) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	// 小型 case-insensitive contains，避免引入 strings.EqualFold + 子字串組合的笨拙寫法。
	ls, lsub := []rune(s), []rune(sub)
	if len(lsub) == 0 {
		return true
	}
	for i := 0; i+len(lsub) <= len(ls); i++ {
		match := true
		for j := 0; j < len(lsub); j++ {
			a, b := ls[i+j], lsub[j]
			if 'A' <= a && a <= 'Z' {
				a += 'a' - 'A'
			}
			if 'A' <= b && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
