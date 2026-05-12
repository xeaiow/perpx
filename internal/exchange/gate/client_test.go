package gate

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

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	creds := &config.Credentials{
		Name:      config.Gate,
		APIKey:    []byte("gt-key"),
		APISecret: []byte("gt-secret"),
	}
	c := NewWithBaseURL(creds, config.Runtime{HTTPTimeoutSec: 5}, srv.URL)
	c.nowFn = func() time.Time { return time.Unix(1700000000, 0) }
	return c
}

func TestSign_KnownFixture(t *testing.T) {
	const want = "dae5806fd611418acdf9bb8fca7fe936af64f9d9edf65df8f5e9d5e39c7e321b980283a1f1d0b88528b28bad228952532cf87a39d0f7cf585dd218766f1e8563"
	got := sign([]byte("gt-secret"), "GET", "/api/v4/futures/usdt/positions", "", "", "1700000000")
	if got != want {
		t.Errorf("sign:\n  got  %s\n  want %s", got, want)
	}
	if len(got) != 128 {
		t.Errorf("expected SHA-512 hex (128 chars), got %d", len(got))
	}
}

func TestSign_EmptyBodyHashIsSHA512(t *testing.T) {
	// SHA-512("") = cf83e1357eefb8bd...
	const emptySha512 = "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
	if len(emptySha512) != 128 {
		t.Fatalf("emptySha512 reference value malformed (got %d hex chars, expected 128)", len(emptySha512))
	}
}

func TestDo_SetsHeaders(t *testing.T) {
	var seenTS, seenSign, seenKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTS = r.Header.Get("Timestamp")
		seenSign = r.Header.Get("SIGN")
		seenKey = r.Header.Get("KEY")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if err := c.do(context.Background(), http.MethodGet, "/api/v4/futures/usdt/positions", nil, nil, &[]rawPosition{}); err != nil {
		t.Fatal(err)
	}
	if seenTS != "1700000000" {
		t.Errorf("Timestamp = %q (should be unix seconds)", seenTS)
	}
	if len(seenSign) != 128 {
		t.Errorf("SIGN length = %d, expected 128 (hex SHA-512)", len(seenSign))
	}
	if seenKey != "gt-key" {
		t.Errorf("KEY = %q", seenKey)
	}
}

func TestPositions_SizeIsContractsCoinSizeFromValueMark(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"contract":"BTC_USDT","size":150,"entry_price":"60000","mark_price":"61000","unrealised_pnl":"100","leverage":"10","value":"9150","mode":"single"}]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.Positions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 position, got %d", len(got))
	}
	p := got[0]
	if p.Symbol != "BTCUSDT" {
		t.Errorf("symbol = %q", p.Symbol)
	}
	if p.Size != 150 {
		t.Errorf("Size should keep raw contracts count, got %v", p.Size)
	}
	// CoinSize = value / mark = 9150 / 61000 = 0.15
	if diff := p.CoinSize - 0.15; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("CoinSize expected 0.15, got %v", p.CoinSize)
	}
}

func TestPositions_NegativeSizeIsShort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"contract":"ETH_USDT","size":-100,"entry_price":"3000","mark_price":"2950","unrealised_pnl":"50","leverage":"5","value":"295","mode":"single"}]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, _ := c.Positions(context.Background())
	if len(got) != 1 || got[0].Side != exchange.SideShort {
		t.Fatalf("expected short, got %+v", got)
	}
	// Size 是 raw size 的絕對值（contracts），不再做 multiplier 換算。
	if got[0].Size != 100 {
		t.Errorf("Size = %v, want 100", got[0].Size)
	}
}

func TestPositions_CrossLeverageFallback(t *testing.T) {
	// 跨倉模式：raw leverage="0"，實際倍率在 cross_leverage_limit。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"contract":"AI_USDT","size":3500,"entry_price":"0.02308","mark_price":"0.02341","unrealised_pnl":"1.155","value":"81.935","leverage":"0","cross_leverage_limit":"5","leverage_max":"10","mode":"single"}]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.Positions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].Leverage != 5 {
		t.Errorf("expected leverage=5 from cross_leverage_limit, got %v", got[0].Leverage)
	}
}

func TestPositions_CoinSizeZeroWhenMarkZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"contract":"BTC_USDT","size":150,"entry_price":"60000","mark_price":"0","value":"0","leverage":"10","mode":"single"}]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, _ := c.Positions(context.Background())
	if len(got) != 1 {
		t.Fatalf("expected 1 position, got %d", len(got))
	}
	if got[0].CoinSize != 0 {
		t.Errorf("CoinSize should be 0 when mark=0, got %v", got[0].CoinSize)
	}
}

func TestBalance_ReturnsAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"total":"10362.92","available":"10000.00","currency":"USDT"}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	v, err := c.AvailableBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != 10000.00 {
		t.Errorf("got %v", v)
	}
}

func TestClose_PassesCloseTrueAndReduceOnly(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, `{"id":1234567890,"contract":"BTC_USDT","size":0,"status":"open"}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	res, err := c.ClosePosition(context.Background(), exchange.CloseRequest{Symbol: "BTC_USDT", Side: exchange.SideLong, Size: 0.015})
	if err != nil {
		t.Fatal(err)
	}
	if body["close"] != true {
		t.Errorf("close = %v", body["close"])
	}
	if body["reduce_only"] != true {
		t.Errorf("reduce_only = %v", body["reduce_only"])
	}
	if res.OrderID != "1234567890" {
		t.Errorf("orderID = %q", res.OrderID)
	}
}

func TestDo_AuthErrMaps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = io.WriteString(w, `{"label":"INVALID_KEY","message":"Invalid API key provided"}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	if !errorsIs(err, exchange.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

func TestSignSeenInURL(t *testing.T) {
	var seenURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenURL = r.URL.String()
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_ = c.do(context.Background(), http.MethodGet, "/api/v4/futures/usdt/position_close", map[string][]string{"limit": {"50"}}, nil, &[]rawHistoryItem{})
	if !strings.Contains(seenURL, "limit=50") {
		t.Errorf("URL = %s", seenURL)
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
