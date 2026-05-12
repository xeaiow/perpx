// Package gate 實作 Gate.io V4 Futures (USDT-settled) adapter。
//
// 注意：Gate 的時間戳是 unix seconds（非 ms）、雜湊是 SHA-512（非 SHA-256），
// size 是 contracts 數（非 coin units），符號用底線分隔（BTC_USDT）。
package gate

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange"
)

const (
	mainnetBaseURL = "https://api.gateio.ws"
	testnetBaseURL = "https://fx-api-testnet.gateio.ws"
)

type Client struct {
	apiKey    []byte
	apiSecret []byte
	baseURL   string
	http      *http.Client

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
	}
}

func (c *Client) Name() string { return "gate" }

func (c *Client) now() time.Time {
	if c.nowFn != nil {
		return c.nowFn()
	}
	return time.Now()
}

func (c *Client) nowSeconds() string {
	return strconv.FormatInt(c.now().Unix(), 10)
}

// sign 計算 Gate V4 簽章。
//
//	hashedBody = hex(SHA512(body))
//	signString = method + "\n" + requestPath + "\n" + queryString + "\n" + hashedBody + "\n" + timestamp
//	sign       = hex(HMAC-SHA512(signString, secret))
func sign(secret []byte, method, requestPath, queryString, body, ts string) string {
	bodyHash := sha512.Sum512([]byte(body))
	bodyHex := hex.EncodeToString(bodyHash[:])
	signString := method + "\n" + requestPath + "\n" + queryString + "\n" + bodyHex + "\n" + ts
	h := hmac.New(sha512.New, secret)
	h.Write([]byte(signString))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body []byte, out any) error {
	ts := c.nowSeconds()
	queryString := ""
	if query != nil {
		queryString = query.Encode()
	}
	bodyStr := ""
	if method == http.MethodPost && len(body) > 0 {
		bodyStr = string(body)
	}
	signature := sign(c.apiSecret, method, path, queryString, bodyStr, ts)

	fullURL := c.baseURL + path
	if queryString != "" {
		fullURL += "?" + queryString
	}
	var reqBody io.Reader
	if bodyStr != "" {
		reqBody = strings.NewReader(bodyStr)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("KEY", string(c.apiKey))
	req.Header.Set("Timestamp", ts)
	req.Header.Set("SIGN", signature)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("gate %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == 429:
		return fmt.Errorf("gate %s: %w", path, exchange.ErrRateLimit)
	case resp.StatusCode >= 500:
		return fmt.Errorf("gate %s: %w: status %d", path, exchange.ErrServerSide, resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		var e rawErr
		_ = json.Unmarshal(rb, &e)
		switch e.Label {
		case "INVALID_KEY", "INVALID_SIGNATURE", "MISSING_REQUIRED_HEADER", "INVALID_CREDENTIALS":
			return fmt.Errorf("gate %s: %w: %s", path, exchange.ErrAuth, e.Message)
		case "TOO_MANY_REQUESTS":
			return fmt.Errorf("gate %s: %w: %s", path, exchange.ErrRateLimit, e.Message)
		default:
			return fmt.Errorf("gate %s: label=%s msg=%s", path, e.Label, e.Message)
		}
	}

	if out != nil {
		if err := json.Unmarshal(rb, out); err != nil {
			return fmt.Errorf("%w: gate %s: %v", exchange.ErrParseResp, path, err)
		}
	}
	return nil
}
