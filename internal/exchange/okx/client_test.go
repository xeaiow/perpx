package okx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange"
)

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	creds := &config.Credentials{
		Name:       config.OKX,
		APIKey:     []byte("ok-key"),
		APISecret:  []byte("ok-secret"),
		Passphrase: []byte("ok-passphrase"),
	}
	c := NewWithBaseURL(creds, config.Runtime{HTTPTimeoutSec: 5}, srv.URL)
	c.nowFn = func() time.Time { return time.Date(2024, 5, 12, 9, 8, 57, 715_000_000, time.UTC) }
	c.synced = true
	return c
}

func TestSign_ISOTimestampFormat(t *testing.T) {
	creds := &config.Credentials{
		APIKey: []byte("k"), APISecret: []byte("s"), Passphrase: []byte("p"),
	}
	c := New(creds, config.Runtime{})
	c.nowFn = func() time.Time { return time.Date(2024, 5, 12, 9, 8, 57, 715_000_000, time.UTC) }
	c.synced = true
	got := c.nowISO()
	want := "2024-05-12T09:08:57.715Z"
	if got != want {
		t.Errorf("ISO ts:\n  got  %s\n  want %s", got, want)
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$`).MatchString(got) {
		t.Errorf("format mismatch")
	}
}

func TestSign_KnownFixture(t *testing.T) {
	// 預期值由獨立 HMAC-SHA256 + base64 計算驗證（見 client_test.go README 內附腳本）。
	// payload = ts + method + requestPath + body
	ts := "2024-05-12T09:08:57.715Z"
	got := sign([]byte("ok-secret"), ts, "GET", "/api/v5/account/positions?instType=SWAP", "")
	const want = "19ZZfAZ+a49I9yXdBxQJjFZkWqMp0klw1mX53daHIzE="
	if got != want {
		t.Errorf("sign:\n  got  %s\n  want %s", got, want)
	}
}

func TestDo_GETIncludesQueryInPath(t *testing.T) {
	var (
		seenURL  string
		seenTs   string
		seenSign string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenURL = r.URL.String()
		seenTs = r.Header.Get("OK-ACCESS-TIMESTAMP")
		seenSign = r.Header.Get("OK-ACCESS-SIGN")
		_, _ = io.WriteString(w, `{"code":"0","msg":"","data":[]}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	q := map[string][]string{"instType": {"SWAP"}}
	if err := c.do(context.Background(), http.MethodGet, "/api/v5/account/positions", q, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(seenURL, "/api/v5/account/positions?instType=SWAP") {
		t.Errorf("URL = %s", seenURL)
	}
	if seenTs == "" || seenSign == "" {
		t.Error("missing headers")
	}
	if !regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`).MatchString(seenTs) {
		t.Errorf("ts not ISO: %s", seenTs)
	}
}

func TestNormalizeSymbol(t *testing.T) {
	cases := map[string]string{
		"BTC-USDT-SWAP": "BTCUSDT",
		"ETH-USDT-SWAP": "ETHUSDT",
		"SOL-USDT":      "SOLUSDT",
	}
	for in, want := range cases {
		if got := normalizeSymbol(in); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
}

func TestDo_AuthErrMaps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":"50113","msg":"Invalid signature","data":[]}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	if !errorsIs(err, exchange.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

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
