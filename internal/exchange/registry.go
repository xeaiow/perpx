package exchange

import (
	"fmt"

	"github.com/yourname/poscli/internal/config"
)

// Registry 是 ExchangeName → Exchange 的 map，用於 UI/CLI 統一存取所有啟用的 adapter。
type Registry map[config.ExchangeName]Exchange

// Adapter 工廠函式。M2 階段只註冊 Bybit；後續 milestone 會 import 各自 package 並擴充。
//
// 之所以用全域 map 而不是每個 adapter init() 自己註冊，是為了讓編譯期就能看到所有
// 註冊狀況（迴圈依賴與雜亂 init 比較難 debug）。
var factories = map[config.ExchangeName]Factory{}

// Factory 接收解密好的 credentials 與 runtime 設定，回傳實作 Exchange 的物件。
type Factory func(c *config.Credentials, rt config.Runtime) (Exchange, error)

// Register 給各個 adapter 套件在 init 期間呼叫。重覆註冊會 panic（程式設計錯誤）。
func Register(name config.ExchangeName, f Factory) {
	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("exchange: %s already registered", name))
	}
	factories[name] = f
}

// NewRegistry 依 LoadResult 建立所有啟用交易所的 adapter。
// 若任一 adapter 建構失敗，整個 NewRegistry 回錯（避免半成品）。
func NewRegistry(r *config.LoadResult) (Registry, error) {
	out := make(Registry)
	for name, creds := range r.Credentials {
		f, ok := factories[name]
		if !ok {
			// 還沒實作的交易所：先 skip 不要 fail，這樣 M2 階段只註冊 bybit
			// 其他交易所即使在 config 裡開啟也只是不顯示。
			continue
		}
		ex, err := f(creds, r.Config.Runtime)
		if err != nil {
			return nil, fmt.Errorf("init %s adapter: %w", name, err)
		}
		out[name] = ex
	}
	return out, nil
}
