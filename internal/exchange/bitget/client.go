// Package bitget 實作 Bitget V2 Mix (USDT-M Futures) adapter。
//
// 簽章與 OKX 雷同：base64(HMAC-SHA256) on (ts + method + requestPath + body)。
// 差異：timestamp 是 unix ms 字串、成功碼是 "00000"。
package bitget

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

const baseURL = "https://api.bitget.com"

type Client struct {
	apiKey     []byte
	apiSecret  []byte
	passphrase []byte
	baseURL    string
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
		baseURL:    baseURL,
		http:       &http.Client{Timeout: timeout},
	}
}

func NewWithBaseURL(c *config.Credentials, rt config.Runtime, base string) *Client {
	cli := New(c, rt)
	cli.baseURL = strings.TrimRight(base, "/")
	return cli
}

func (c *Client) Name() string { return "bitget" }

func (c *Client) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn()
	}
	return time.Now()
}

func (c *Client) nowMs() string {
	c.mu.Lock()
	delta := c.timeDelta
	c.mu.Unlock()
	return strconv.FormatInt(c.now().Add(delta).UnixMilli(), 10)
}

func (c *Client) syncTime(ctx context.Context) error {
	c.mu.Lock()
	if c.synced {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	u := c.baseURL + "/api/v2/public/time"
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
		return err
	}
	raw, _ := json.Marshal(env.Data)
	var st rawServerTime
	_ = json.Unmarshal(raw, &st)
	serverMs, _ := strconv.ParseInt(st.ServerTime, 10, 64)
	if serverMs == 0 && env.RequestTime > 0 {
		serverMs = env.RequestTime
	}
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

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, out any) error {
	_ = c.syncTime(ctx)

	retried := false
retry:
	ts := c.nowMs()
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
	req.Header.Set("ACCESS-KEY", string(c.apiKey))
	req.Header.Set("ACCESS-SIGN", signature)
	req.Header.Set("ACCESS-PASSPHRASE", string(c.passphrase))
	req.Header.Set("ACCESS-TIMESTAMP", ts)
	req.Header.Set("locale", "en-US")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("bitget %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == 429:
		return fmt.Errorf("bitget %s: %w", path, exchange.ErrRateLimit)
	case resp.StatusCode >= 500:
		return fmt.Errorf("bitget %s: %w: status %d", path, exchange.ErrServerSide, resp.StatusCode)
	case resp.StatusCode == 401 || resp.StatusCode == 403:
		return fmt.Errorf("bitget %s: %w: status %d", path, exchange.ErrAuth, resp.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(rb, &env); err != nil {
		return fmt.Errorf("%w: bitget %s: %v", exchange.ErrParseResp, path, err)
	}
	if env.Code != "00000" {
		switch env.Code {
		case "40010":
			if !retried {
				c.mu.Lock()
				c.synced = false
				c.mu.Unlock()
				_ = c.syncTime(ctx)
				retried = true
				goto retry
			}
			return fmt.Errorf("bitget %s: %w: %s", path, exchange.ErrAuth, env.Msg)
		case "40006", "40009", "40015":
			return fmt.Errorf("bitget %s: %w: %s", path, exchange.ErrAuth, env.Msg)
		default:
			if strings.HasPrefix(env.Code, "429") {
				return fmt.Errorf("bitget %s: %w: %s", path, exchange.ErrRateLimit, env.Msg)
			}
			return fmt.Errorf("bitget %s: code=%s msg=%s", path, env.Code, env.Msg)
		}
	}

	if out != nil {
		raw, _ := json.Marshal(env.Data)
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%w: bitget %s data: %v", exchange.ErrParseResp, path, err)
		}
	}
	return nil
}
