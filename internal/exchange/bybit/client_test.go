package bybit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange"
)

// 建一個固定時間 + testServer 為基礎的客戶端，給整套測試重用。
func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	creds := &config.Credentials{
		Name:      config.Bybit,
		APIKey:    []byte("VPm8wWcYAhERmkPC83"),
		APISecret: []byte("rh2H1HhJWslsl0dG2W1QxvxsM1ulfPLb3LMq"),
	}
	c := NewWithBaseURL(creds, config.Runtime{HTTPTimeoutSec: 5}, srv.URL, "/v5")
	// 固定本地時間，避免 time sync 後 delta 漂移影響 signature 測試
	c.nowFn = func() time.Time { return time.UnixMilli(1672382213281) }
	// 直接設定 synced，避免每次測試都得 stub /market/time
	c.synced = true
	return c
}

// TestSign_KnownFixture：簽章用 Bybit 文件的固定 payload 算出來，避免將來改動破壞簽章邏輯。
func TestSign_KnownFixture(t *testing.T) {
	apiKey := "VPm8wWcYAhERmkPC83"
	apiSecret := []byte("rh2H1HhJWslsl0dG2W1QxvxsM1ulfPLb3LMq")
	ts := "1672382213281"
	recvWindow := "5000"
	payload := "accountType=UNIFIED"

	got := sign(apiSecret, ts, apiKey, recvWindow, payload)

	// 預期值由獨立 HMAC-SHA256 計算工具產出，驗證 sign() 沒寫錯。
	// payload = "1672382213281VPm8wWcYAhERmkPC835000accountType=UNIFIED"
	// secret  = "rh2H1HhJWslsl0dG2W1QxvxsM1ulfPLb3LMq"
	const want = "e86a304d7061a47737eca4bab64733857325e276970f703a53d5e799e489c4bc"
	if got != want {
		t.Fatalf("sign mismatch:\n  got  %s\n  want %s", got, want)
	}
}

func TestSign_DifferentSecretsDiffer(t *testing.T) {
	a := sign([]byte("s1"), "1", "k", "5000", "x")
	b := sign([]byte("s2"), "1", "k", "5000", "x")
	if a == b {
		t.Fatal("expected different signatures for different secrets")
	}
}

// TestDo_GETSendsCorrectHeaders 驗證簽章 header / query 上得去。
func TestDo_GETSendsCorrectHeaders(t *testing.T) {
	var seen *http.Request
	var seenURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r
		seenURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{},"time":1}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	q := map[string][]string{
		"category":   {"linear"},
		"settleCoin": {"USDT"},
	}
	if err := c.do(context.Background(), http.MethodGet, "/position/list", q, nil, nil); err != nil {
		t.Fatalf("do: %v", err)
	}
	if seen == nil {
		t.Fatal("server saw no request")
	}
	if seen.Header.Get("X-BAPI-API-KEY") == "" {
		t.Error("X-BAPI-API-KEY missing")
	}
	if seen.Header.Get("X-BAPI-SIGN") == "" {
		t.Error("X-BAPI-SIGN missing")
	}
	if seen.Header.Get("X-BAPI-TIMESTAMP") == "" {
		t.Error("X-BAPI-TIMESTAMP missing")
	}
	if seen.Header.Get("X-BAPI-RECV-WINDOW") != "5000" {
		t.Errorf("recv window = %q", seen.Header.Get("X-BAPI-RECV-WINDOW"))
	}
	if !strings.Contains(seenURL, "/v5/position/list") {
		t.Errorf("expected /v5/position/list in URL, got %s", seenURL)
	}
	if !strings.Contains(seenURL, "category=linear") {
		t.Errorf("expected query string with category=linear, got %s", seenURL)
	}
}

func TestDo_AuthErrorMapsToErrAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"retCode":10003,"retMsg":"Invalid API key","result":null}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/anything", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errorsIs(err, exchange.ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}

func TestDo_RateLimitMapsToErrRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"retCode":10006,"retMsg":"Too many visits","result":null}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	if !errorsIs(err, exchange.ErrRateLimit) {
		t.Fatalf("expected ErrRateLimit, got %v", err)
	}
}

func TestDo_HTTPStatus429MapsToRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	if !errorsIs(err, exchange.ErrRateLimit) {
		t.Fatalf("expected ErrRateLimit on 429, got %v", err)
	}
}

func TestDo_TimeSyncRetryOnRecvWindowErr(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v5/market/time" {
			_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"timeSecond":"1672382213"},"time":1672382213281}`)
			return
		}
		calls++
		if calls == 1 {
			_, _ = io.WriteString(w, `{"retCode":10002,"retMsg":"timestamp out of recv_window","result":null}`)
			return
		}
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[]},"time":1672382213281}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	c.synced = false // 強制走 /market/time
	var out rawPositionList
	if err := c.do(context.Background(), http.MethodGet, "/position/list", nil, nil, &out); err != nil {
		t.Fatalf("do: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls (first 10002, retry), got %d", calls)
	}
}

func TestDo_TimeSyncOnlyOncePerClient(t *testing.T) {
	var timeCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v5/market/time" {
			timeCalls++
			_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"timeSecond":"1672382213"},"time":1672382213281}`)
			return
		}
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[]},"time":1672382213281}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	c.synced = false
	for i := 0; i < 3; i++ {
		_ = c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	}
	if timeCalls != 1 {
		t.Fatalf("expected exactly 1 time-sync call, got %d", timeCalls)
	}
}

// 不直接 import errors 避免額外引用；用 helper。
func errorsIs(err, target error) bool {
	for cur := err; cur != nil; {
		if cur == target {
			return true
		}
		u, ok := cur.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		cur = u.Unwrap()
	}
	return false
}

// JSON sanity helper used in tests below.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
