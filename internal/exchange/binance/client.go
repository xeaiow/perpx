// Package binance 實作 Binance USDⓈ-M Futures adapter。
//
// 簽章：HMAC-SHA256 over 完整 query string 或 form body；signature 以 &signature= 形式 append。
// Header X-MBX-APIKEY 帶 API key。POST 用 application/x-www-form-urlencoded。
package binance

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
	mainnetBaseURL = "https://fapi.binance.com"
	testnetBaseURL = "https://testnet.binancefuture.com"
	defaultRecvWin = "5000"
)

type Client struct {
	apiKey    []byte
	apiSecret []byte
	baseURL   string
	http      *http.Client
	recvWin   string

	mu        sync.Mutex
	timeDelta time.Duration
	synced    bool

	nowFn func() time.Time
}

func New(c *config.Credentials, rt config.Runtime) *Client {
	base := mainnetBaseURL
	if rt.UseTestnet {
		base = testnetBaseURL
	}
	return NewWithBaseURL(c, rt, base)
}

func NewWithBaseURL(c *config.Credentials, rt config.Runtime, base string) *Client {
	timeout := time.Duration(rt.HTTPTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		apiKey:    append([]byte(nil), c.APIKey...),
		apiSecret: append([]byte(nil), c.APISecret...),
		baseURL:   strings.TrimRight(base, "/"),
		http:      &http.Client{Timeout: timeout},
		recvWin:   defaultRecvWin,
	}
}

func (c *Client) Name() string { return "binance" }

func (c *Client) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn()
	}
	return time.Now()
}

func (c *Client) nowMs() int64 {
	c.mu.Lock()
	delta := c.timeDelta
	c.mu.Unlock()
	return c.now().Add(delta).UnixMilli()
}

func (c *Client) syncTime(ctx context.Context) error {
	c.mu.Lock()
	if c.synced {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/fapi/v1/time", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var t rawTime
	if err := json.Unmarshal(body, &t); err != nil {
		c.mu.Lock()
		c.synced = true
		c.mu.Unlock()
		return nil
	}
	c.mu.Lock()
	c.timeDelta = time.Duration(t.ServerTime-c.now().UnixMilli()) * time.Millisecond
	c.synced = true
	c.mu.Unlock()
	return nil
}

func sign(secret []byte, payload string) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// do 執行已簽章請求。
// params 必須包含所有要送到 server 的參數（不含 timestamp/signature；本函式自己加）。
// GET：params encode 後變成 query；POST：params encode 後變成 form body。
func (c *Client) do(ctx context.Context, method, path string, params url.Values, signedRequest bool, out any) error {
	_ = c.syncTime(ctx)

	retried := false
retry:
	if params == nil {
		params = url.Values{}
	}
	var bodyStr string
	var fullURL string
	if signedRequest {
		params.Set("timestamp", strconv.FormatInt(c.nowMs(), 10))
		params.Set("recvWindow", c.recvWin)
		encoded := params.Encode()
		signature := sign(c.apiSecret, encoded)
		encoded = encoded + "&signature=" + signature
		switch method {
		case http.MethodGet, http.MethodDelete:
			fullURL = c.baseURL + path + "?" + encoded
		case http.MethodPost:
			bodyStr = encoded
			fullURL = c.baseURL + path
		}
	} else {
		// 未簽章請求：純 GET 加 query。
		if q := params.Encode(); q != "" {
			fullURL = c.baseURL + path + "?" + q
		} else {
			fullURL = c.baseURL + path
		}
	}

	var reqBody io.Reader
	if bodyStr != "" {
		reqBody = strings.NewReader(bodyStr)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return err
	}
	if len(c.apiKey) > 0 {
		req.Header.Set("X-MBX-APIKEY", string(c.apiKey))
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("binance %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == 429 || resp.StatusCode == 418:
		return fmt.Errorf("binance %s: %w", path, exchange.ErrRateLimit)
	case resp.StatusCode >= 500:
		return fmt.Errorf("binance %s: %w: status %d", path, exchange.ErrServerSide, resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		var e rawErr
		_ = json.Unmarshal(rb, &e)
		switch e.Code {
		case -1021:
			if !retried {
				c.mu.Lock()
				c.synced = false
				c.mu.Unlock()
				_ = c.syncTime(ctx)
				retried = true
				params.Del("timestamp")
				params.Del("recvWindow")
				goto retry
			}
			return fmt.Errorf("binance %s: %w: %s", path, exchange.ErrAuth, e.Msg)
		case -1022, -2014, -2015:
			return fmt.Errorf("binance %s: %w: %s", path, exchange.ErrAuth, e.Msg)
		case -1003:
			return fmt.Errorf("binance %s: %w: %s", path, exchange.ErrRateLimit, e.Msg)
		default:
			if resp.StatusCode == 401 || resp.StatusCode == 403 {
				return fmt.Errorf("binance %s: %w: %s", path, exchange.ErrAuth, e.Msg)
			}
			return fmt.Errorf("binance %s: code=%d msg=%s", path, e.Code, e.Msg)
		}
	}

	if out != nil {
		if err := json.Unmarshal(rb, out); err != nil {
			return fmt.Errorf("%w: binance %s: %v", exchange.ErrParseResp, path, err)
		}
	}
	return nil
}
