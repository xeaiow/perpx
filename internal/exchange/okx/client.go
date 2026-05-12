// Package okx 實作 OKX V5 USDT-M 永續合約 adapter。
//
// 注意：OKX 的時間戳用 ISO 8601 (帶毫秒) 而非 unix ms；簽章用 base64(HMAC-SHA256)；
// passphrase 為必填 header；Content-Type 永遠 application/json。
package okx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
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
	mainnetBaseURL = "https://www.okx.com"
	// OKX 沒有獨立 testnet 網域，demo 改用同網域加 header；先保留欄位將來用
)

// Client 是 OKX V5 adapter。
type Client struct {
	apiKey     []byte
	apiSecret  []byte
	passphrase []byte
	baseURL    string
	useDemo    bool
	http       *http.Client

	mu        sync.Mutex
	timeDelta time.Duration
	synced    bool

	nowFn func() time.Time
}

func New(c *config.Credentials, rt config.Runtime) *Client {
	timeout := time.Duration(rt.HTTPTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		apiKey:     append([]byte(nil), c.APIKey...),
		apiSecret:  append([]byte(nil), c.APISecret...),
		passphrase: append([]byte(nil), c.Passphrase...),
		baseURL:    mainnetBaseURL,
		useDemo:    rt.UseTestnet,
		http:       &http.Client{Timeout: timeout},
	}
}

// NewWithBaseURL 給測試用。
func NewWithBaseURL(c *config.Credentials, rt config.Runtime, baseURL string) *Client {
	cli := New(c, rt)
	cli.baseURL = strings.TrimRight(baseURL, "/")
	return cli
}

func (c *Client) Name() string { return "okx" }

func (c *Client) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn()
	}
	return time.Now()
}

// nowISO 回傳 ISO 8601 毫秒 UTC（OKX 規定格式）。
func (c *Client) nowISO() string {
	c.mu.Lock()
	delta := c.timeDelta
	c.mu.Unlock()
	return c.now().UTC().Add(delta).Format("2006-01-02T15:04:05.000Z")
}

func (c *Client) syncTime(ctx context.Context) error {
	c.mu.Lock()
	if c.synced {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	u := c.baseURL + "/api/v5/public/time"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("%w: okx time: %v", exchange.ErrParseResp, err)
	}
	// data 是陣列 [{ts}]
	raw, _ := json.Marshal(env.Data)
	var arr []rawServerTime
	if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
		c.mu.Lock()
		c.synced = true
		c.mu.Unlock()
		return nil
	}
	serverMs, _ := strconv.ParseInt(arr[0].Ts, 10, 64)
	c.mu.Lock()
	c.timeDelta = time.Duration(serverMs-c.now().UnixMilli()) * time.Millisecond
	c.synced = true
	c.mu.Unlock()
	return nil
}

func sign(secret []byte, ts, method, requestPath, body string) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(ts))
	h.Write([]byte(method))
	h.Write([]byte(requestPath))
	h.Write([]byte(body))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// do 執行簽章請求。
// GET：requestPath 含 query；body=""。
// POST：query 必須為 nil；body 為已 marshal 的 JSON。
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, out any) error {
	_ = c.syncTime(ctx) // 同步失敗不阻斷

	retried := false
retry:
	ts := c.nowISO()
	requestPath := path
	if method == http.MethodGet && query != nil {
		if q := query.Encode(); q != "" {
			requestPath = path + "?" + q
		}
	}
	bodyStr := ""
	if method == http.MethodPost && len(body) > 0 {
		bodyStr = string(body)
	}
	signature := sign(c.apiSecret, ts, method, requestPath, bodyStr)

	fullURL := c.baseURL + requestPath
	var reqBody io.Reader
	if bodyStr != "" {
		reqBody = strings.NewReader(bodyStr)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("OK-ACCESS-KEY", string(c.apiKey))
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
	req.Header.Set("OK-ACCESS-PASSPHRASE", string(c.passphrase))
	req.Header.Set("Content-Type", "application/json")
	if c.useDemo {
		req.Header.Set("x-simulated-trading", "1")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("okx %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == 429:
		return fmt.Errorf("okx %s: %w", path, exchange.ErrRateLimit)
	case resp.StatusCode >= 500:
		return fmt.Errorf("okx %s: %w: status %d", path, exchange.ErrServerSide, resp.StatusCode)
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return fmt.Errorf("okx %s: %w: status %d", path, exchange.ErrAuth, resp.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(rb, &env); err != nil {
		return fmt.Errorf("%w: okx %s: %v", exchange.ErrParseResp, path, err)
	}
	if env.Code != "0" {
		switch env.Code {
		case "50102":
			if !retried {
				c.mu.Lock()
				c.synced = false
				c.mu.Unlock()
				_ = c.syncTime(ctx)
				retried = true
				goto retry
			}
			return fmt.Errorf("okx %s: %w: %s", path, exchange.ErrAuth, env.Msg)
		case "50111", "50113", "50114":
			return fmt.Errorf("okx %s: %w: %s", path, exchange.ErrAuth, env.Msg)
		case "50011":
			return fmt.Errorf("okx %s: %w: %s", path, exchange.ErrRateLimit, env.Msg)
		default:
			return fmt.Errorf("okx %s: code=%s msg=%s", path, env.Code, env.Msg)
		}
	}

	if out != nil {
		raw, _ := json.Marshal(env.Data)
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%w: okx %s data: %v", exchange.ErrParseResp, path, err)
		}
	}
	return nil
}

// normalizeSymbol：BTC-USDT-SWAP → BTCUSDT
func normalizeSymbol(instID string) string {
	s := strings.TrimSuffix(instID, "-SWAP")
	return strings.ReplaceAll(s, "-", "")
}
