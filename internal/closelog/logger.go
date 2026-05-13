// Package closelog 為平倉操作提供結構化 JSON log。
//
// 寫入 ~/.config/poscli/close.log（或 $POSCLI_LOG_FILE 覆寫）；
// 以 lumberjack 控制大小（10MB rotate、保留 5 個備份）。
//
// 不暴露 zap 物件給呼叫端；提供四個語意函式 Requested / Completed /
// Failed / Cancelled，呼叫端只需丟欄位進來。多 goroutine 安全。
package closelog

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultRelPath = ".config/poscli/close.log"
	// EnvOverride 是支援 $POSCLI_LOG_FILE 的 env var 名稱。main.go 在組合
	// flag > env > config > default 優先級時會用到這個常數。
	EnvOverride = "POSCLI_LOG_FILE"

	maxSizeMB  = 10
	maxBackups = 5
	maxAgeDays = 0 // 不依年齡刪、只看大小
	compress   = false
)

var (
	mu        sync.Mutex
	current   *zap.Logger
	currentTo string // 目前 logger 寫到的檔案路徑（給測試確認）
)

// Init 初始化 logger 並把寫入位置設為 path。
// 失敗時不阻斷程式啟動、改回 nop logger；path 為空字串時退到 DefaultPath()。
// 多次呼叫會以最新一次為準（測試方便、生產上呼叫一次即可）。
func Init(path string) {
	mu.Lock()
	defer mu.Unlock()
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		current = zap.NewNop()
		currentTo = ""
		return
	}
	w := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    maxSizeMB,
		MaxBackups: maxBackups,
		MaxAge:     maxAgeDays,
		Compress:   compress,
	}
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(w),
		zapcore.InfoLevel,
	)
	current = zap.New(core)
	currentTo = path
}

// InitWithPath 是 Init 的別名（向後相容；舊測試呼叫的是這個）。
// 回傳 error 是為了能在測試中表達 mkdir 失敗、目前 Init 把錯誤吞了。
func InitWithPath(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	Init(path)
	return nil
}

// DefaultPath 回傳預設 log 位置：~/.config/poscli/close.log。
// HOME 無法取得時退到當前工作目錄下的 close.log。
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "close.log"
	}
	return filepath.Join(home, defaultRelPath)
}

// Sync 把 buffer 寫出（在 main 結尾 defer 一次）。
func Sync() {
	mu.Lock()
	l := current
	mu.Unlock()
	if l != nil {
		_ = l.Sync()
	}
}

func logger() *zap.Logger {
	mu.Lock()
	defer mu.Unlock()
	if current == nil {
		return zap.NewNop()
	}
	return current
}

// Fields 是呼叫端要傳進來的倉位 snapshot。值少於四個事件、
// 每個事件記錄欄位略有不同；用一個 struct 傳乾淨些。
type Fields struct {
	Exchange   string
	Symbol     string
	RawSymbol  string
	Side       string
	Size       float64
	CoinSize   float64
	EntryPrice float64
	MarkPrice  float64
	UPnL       float64
	MarginMode string
}

func base(f Fields) []zap.Field {
	return []zap.Field{
		zap.String("exchange", f.Exchange),
		zap.String("symbol", f.Symbol),
		zap.String("rawSymbol", f.RawSymbol),
		zap.String("side", f.Side),
		zap.Float64("size", f.Size),
		zap.Float64("coinSize", f.CoinSize),
		zap.Float64("entry", f.EntryPrice),
		zap.Float64("mark", f.MarkPrice),
		zap.Float64("upnl", f.UPnL),
		zap.String("marginMode", f.MarginMode),
	}
}

// Requested：使用者按 y、close 請求即將送出。
func Requested(f Fields) {
	logger().Info("close.requested", base(f)...)
}

// Completed：close 成功提交（HTTP/envelope 都 OK）。
// orderID 可能為空字串（OKX close-position 不回 OrderID）。
// latency = 從發送到收到回應的耗時。
func Completed(f Fields, orderID string, latency time.Duration) {
	fields := append(base(f),
		zap.String("event", "completed"),
		zap.String("orderID", orderID),
		zap.Int64("latencyMs", latency.Milliseconds()),
	)
	logger().Info("close.completed", fields...)
}

// Failed：close 提交失敗（HTTP / envelope code / timeout 任一）。
func Failed(f Fields, err error, latency time.Duration) {
	fields := append(base(f),
		zap.String("event", "failed"),
		zap.String("error", errString(err)),
		zap.Int64("latencyMs", latency.Milliseconds()),
	)
	logger().Info("close.failed", fields...)
}

// Cancelled：使用者按 n / esc。
func Cancelled(f Fields) {
	logger().Info("close.cancelled", append(base(f), zap.String("event", "cancelled"))...)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

