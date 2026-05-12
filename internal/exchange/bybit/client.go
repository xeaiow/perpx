// Package bybit 實作 Bybit V5 USDT-M 永續合約 adapter。
// 預設使用 mainnet；如 cfg.Runtime.UseTestnet 為 true 則用 testnet。
//
// Zoomex (M3) 透過 NewWithBaseURL 重用本套件的簽章、time sync、do() helper。
package bybit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange"
)

const (
	defaultRecvWindow = "5000"
	defaultPathPrefix = "/v5"

	mainnetBaseURL = "https://api.bybit.com"
	testnetBaseURL = "https://api-testnet.bybit.com"
)

// Client 是 Bybit V5 adapter。
type Client struct {
	apiKey     []byte
	apiSecret  []byte
	baseURL    string
	pathPrefix string
	recvWindow string

	http *http.Client

	// 同伺服器時間偏差，nowMs() 用。
	mu        sync.Mutex
	timeDelta time.Duration // server - local
	synced    bool

	// nowFn 在測試裡注入固定時鐘；nil 時用 time.Now。
	nowFn func() time.Time

	// nameOverride 讓 Zoomex 把 Name() 改為 "zoomex"。空字串時用預設 "bybit"。
	nameOverride string
}

// New 建立預設 Bybit 客戶端。
func New(c *config.Credentials, rt config.Runtime) *Client {
	baseURL := mainnetBaseURL
	if rt.UseTestnet {
		baseURL = testnetBaseURL
	}
	return NewWithBaseURL(c, rt, baseURL, defaultPathPrefix)
}

// NewWithBaseURL 用指定 baseURL 與 pathPrefix 建立客戶端。Zoomex 透過此函式重用本套件。
func NewWithBaseURL(c *config.Credentials, rt config.Runtime, baseURL, pathPrefix string) *Client {
	timeout := time.Duration(rt.HTTPTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	// pathPrefix 標準化：去尾部 /；若不為空且不以 / 開頭則加上。
	pathPrefix = strings.TrimRight(pathPrefix, "/")
	if pathPrefix != "" && !strings.HasPrefix(pathPrefix, "/") {
		pathPrefix = "/" + pathPrefix
	}
	return &Client{
		apiKey:     append([]byte(nil), c.APIKey...),
		apiSecret:  append([]byte(nil), c.APISecret...),
		baseURL:    strings.TrimRight(baseURL, "/"),
		pathPrefix: pathPrefix,
		recvWindow: defaultRecvWindow,
		http:       &http.Client{Timeout: timeout},
	}
}

// SetName 讓 Zoomex 之類的衍生 client 覆寫顯示名稱。
func (c *Client) SetName(name string) { c.nameOverride = name }

// Name 回傳 adapter 名稱；預設 "bybit"，被 SetName 覆寫後不同。
func (c *Client) Name() string {
	if c.nameOverride != "" {
		return c.nameOverride
	}
	return "bybit"
}

func (c *Client) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn()
	}
	return time.Now()
}

// nowMs 回傳本地時間 + delta，毫秒字串。
func (c *Client) nowMs() string {
	c.mu.Lock()
	delta := c.timeDelta
	c.mu.Unlock()
	ms := c.now().Add(delta).UnixMilli()
	return strconv.FormatInt(ms, 10)
}

// syncTime 從 /market/time 取得伺服器時間，更新 delta。
// 失敗就保持本地時間（delta=0）；多次失敗不阻斷請求。
func (c *Client) syncTime(ctx context.Context) error {
	c.mu.Lock()
	if c.synced {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	u := c.baseURL + c.pathPrefix + "/market/time"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("%w: time: %v", exchange.ErrParseResp, err)
	}
	rb, _ := json.Marshal(env.Result)
	var st rawServerTime
	if err := json.Unmarshal(rb, &st); err != nil {
		return fmt.Errorf("%w: time result: %v", exchange.ErrParseResp, err)
	}
	// timeNano 是奈秒字串；env.Time 是 server 的 unix ms。優先用 env.Time。
	var serverMs int64
	if env.Time > 0 {
		serverMs = env.Time
	} else if st.TimeSecond != "" {
		s, _ := strconv.ParseInt(st.TimeSecond, 10, 64)
		serverMs = s * 1000
	}
	c.mu.Lock()
	c.timeDelta = time.Duration(serverMs-c.now().UnixMilli()) * time.Millisecond
	c.synced = true
	c.mu.Unlock()
	return nil
}

// resetTimeSync 強制下次請求前重新同步時間（給 recv-window 錯誤後重試用）。
func (c *Client) resetTimeSync() {
	c.mu.Lock()
	c.synced = false
	c.mu.Unlock()
}

// sign 計算 HMAC-SHA256 簽章。payload = ts + apiKey + recvWindow + (query or body)。
func sign(secret []byte, ts, apiKey, recvWindow, payload string) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(ts))
	h.Write([]byte(apiKey))
	h.Write([]byte(recvWindow))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// do 執行一次簽章請求。method 必須是 GET 或 POST。
// GET：query 帶入；body 必須為 nil。
// POST：body 帶入（已 marshal 的 JSON bytes）；query 必須為 nil。
// out 是 result 欄位 unmarshal 的目標；nil 表示不解。
//
// 若回傳是 recv-window 相關錯誤碼則自動重新同步時間並重試一次。
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, out any) error {
	if err := c.syncTime(ctx); err != nil {
		// 同步失敗不阻斷；後面以本地時間嘗試。
		_ = err
	}

	retried := false
retry:
	ts := c.nowMs()
	var (
		fullURL string
		signPayload string
	)
	switch method {
	case http.MethodGet:
		q := ""
		if query != nil {
			q = query.Encode()
		}
		signPayload = q
		fullURL = c.baseURL + c.pathPrefix + path
		if q != "" {
			fullURL += "?" + q
		}
	case http.MethodPost:
		signPayload = string(body)
		fullURL = c.baseURL + c.pathPrefix + path
	default:
		return fmt.Errorf("%w: unsupported method %s", exchange.ErrClientSide, method)
	}

	signature := sign(c.apiSecret, ts, string(c.apiKey), c.recvWindow, signPayload)

	var reqBody io.Reader
	if method == http.MethodPost && len(body) > 0 {
		reqBody = strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("X-BAPI-API-KEY", string(c.apiKey))
	req.Header.Set("X-BAPI-TIMESTAMP", ts)
	req.Header.Set("X-BAPI-RECV-WINDOW", c.recvWindow)
	req.Header.Set("X-BAPI-SIGN", signature)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bybit %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("bybit read body: %w", err)
	}

	// 先依 HTTP status 處理速率/伺服器層級錯誤
	switch {
	case resp.StatusCode == 429:
		return fmt.Errorf("bybit %s: %w", path, exchange.ErrRateLimit)
	case resp.StatusCode >= 500:
		return fmt.Errorf("bybit %s: %w: status %d", path, exchange.ErrServerSide, resp.StatusCode)
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return fmt.Errorf("bybit %s: %w: status %d", path, exchange.ErrAuth, resp.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(rb, &env); err != nil {
		return fmt.Errorf("%w: bybit %s: %v", exchange.ErrParseResp, path, err)
	}

	if env.RetCode != 0 {
		switch env.RetCode {
		case 10002:
			if !retried {
				c.resetTimeSync()
				_ = c.syncTime(ctx)
				retried = true
				goto retry
			}
			return fmt.Errorf("bybit %s: %w: %s", path, exchange.ErrAuth, env.RetMsg)
		case 10003, 10004, 10005:
			return fmt.Errorf("bybit %s: %w: %s", path, exchange.ErrAuth, env.RetMsg)
		case 10006:
			return fmt.Errorf("bybit %s: %w: %s", path, exchange.ErrRateLimit, env.RetMsg)
		case 10001:
			return fmt.Errorf("bybit %s: %w: %s", path, exchange.ErrClientSide, env.RetMsg)
		default:
			return fmt.Errorf("bybit %s: retCode=%d retMsg=%s", path, env.RetCode, env.RetMsg)
		}
	}

	if out != nil {
		// 把 env.Result 重新 marshal 後 unmarshal 進 out。
		// 比 json.RawMessage 更穩定（result 可能是 array 或 object）。
		raw, _ := json.Marshal(env.Result)
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%w: bybit %s result: %v", exchange.ErrParseResp, path, err)
		}
	}
	return nil
}
