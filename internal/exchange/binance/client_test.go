package binance

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange"
)

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	creds := &config.Credentials{
		Name:      config.Binance,
		APIKey:    []byte("bn-key"),
		APISecret: []byte("bn-secret"),
	}
	c := NewWithBaseURL(creds, config.Runtime{HTTPTimeoutSec: 5}, srv.URL)
	c.nowFn = func() time.Time { return time.UnixMilli(1700000000000) }
	c.synced = true
	return c
}

func TestSign_KnownFixture(t *testing.T) {
	const want = "e4ca4ff898cf9cf564145156ece7a1b21ce1c50c2422df5a4359d813b6c55be5"
	got := sign([]byte("bn-secret"), "timestamp=1700000000000&recvWindow=5000")
	if got != want {
		t.Errorf("sign:\n  got  %s\n  want %s", got, want)
	}
}

func TestDo_GETAppendsSignatureToQuery(t *testing.T) {
	var seenURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenURL = r.URL.String()
		_, _ = io.WriteString(w, `[]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	if err := c.do(context.Background(), http.MethodGet, "/fapi/v3/positionRisk", nil, true, &[]rawPosition{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seenURL, "signature=") {
		t.Errorf("expected signature param, got %s", seenURL)
	}
	if !strings.Contains(seenURL, "timestamp=") {
		t.Errorf("expected timestamp param, got %s", seenURL)
	}
	if !strings.Contains(seenURL, "recvWindow=5000") {
		t.Errorf("expected recvWindow=5000, got %s", seenURL)
	}
}

func TestDo_POSTSendsFormBody(t *testing.T) {
	var (
		ctype string
		body  string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctype = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = io.WriteString(w, `{"orderId":1,"symbol":"BTCUSDT","status":"NEW"}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	p := url.Values{}
	p.Set("symbol", "BTCUSDT")
	p.Set("side", "SELL")
	if err := c.do(context.Background(), http.MethodPost, "/fapi/v1/order", p, true, &rawOrderResp{}); err != nil {
		t.Fatal(err)
	}
	if ctype != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", ctype)
	}
	if !strings.Contains(body, "symbol=BTCUSDT") || !strings.Contains(body, "signature=") {
		t.Errorf("body = %q", body)
	}
}

func TestDo_AuthErrorMaps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = io.WriteString(w, `{"code":-1022,"msg":"Invalid signature"}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/x", nil, true, nil)
	if !errorsIs(err, exchange.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

func TestDo_RateLimitMaps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/x", nil, true, nil)
	if !errorsIs(err, exchange.ErrRateLimit) {
		t.Fatalf("want ErrRateLimit, got %v", err)
	}
}

func TestPositions_NegativeAmtIsShort(t *testing.T) {
	const body = `[
      {"symbol":"BTCUSDT","positionSide":"BOTH","positionAmt":"0.150","entryPrice":"63421.5","markPrice":"64012.30","unRealizedProfit":"88.62","leverage":"10","marginType":"cross","notional":"9601.84","marginAsset":"USDT","updateTime":1720736417660},
      {"symbol":"ETHUSDT","positionSide":"BOTH","positionAmt":"-2.5","entryPrice":"3120","markPrice":"3088","unRealizedProfit":"80","leverage":"5","marginType":"isolated","notional":"-7720","marginAsset":"USDT","updateTime":1720736417661},
      {"symbol":"EMPTY","positionSide":"BOTH","positionAmt":"0","marginAsset":"USDT"}
    ]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.Positions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 (empty filtered), got %d", len(got))
	}
	var eth *exchange.Position
	for i := range got {
		if got[i].Symbol == "ETHUSDT" {
			eth = &got[i]
		}
	}
	if eth == nil || eth.Side != exchange.SideShort || eth.Size != 2.5 {
		t.Errorf("ETH not parsed correctly: %+v", eth)
	}
	if eth.Notional != 7720 {
		t.Errorf("notional should be absolute, got %v", eth.Notional)
	}
}

func TestBalance_FindsUSDT(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[{"asset":"BNB","availableBalance":"1.0"},{"asset":"USDT","availableBalance":"12450.32","balance":"12450.32"}]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	v, err := c.AvailableBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != 12450.32 {
		t.Errorf("got %v", v)
	}
}

func TestHistory_MapsRealizedPnl(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `[
          {"symbol":"BTCUSDT","incomeType":"REALIZED_PNL","income":"88.61","asset":"USDT","time":1684742410020,"tranId":1,"tradeId":"1"},
          {"symbol":"FUND","incomeType":"FUNDING_FEE","income":"-0.10","asset":"USDT","time":1684742410100},
          {"symbol":"OTHER","incomeType":"REALIZED_PNL","income":"3","asset":"BUSD","time":1684742410200}
        ]`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.History(context.Background(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Symbol != "BTCUSDT" {
		t.Errorf("history filter failed: %+v", got)
	}
}

func TestClose_OneWaySendsReduceOnly(t *testing.T) {
	var formBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		formBody = string(b)
		_, _ = io.WriteString(w, `{"orderId":28,"symbol":"BTCUSDT","status":"NEW"}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	res, err := c.ClosePosition(context.Background(), exchange.CloseRequest{Symbol: "BTCUSDT", Side: exchange.SideLong, Size: 0.15})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(formBody, "side=SELL") {
		t.Errorf("expected SELL, body=%s", formBody)
	}
	if !strings.Contains(formBody, "reduceOnly=true") {
		t.Errorf("expected reduceOnly=true, body=%s", formBody)
	}
	if !strings.Contains(formBody, "positionSide=BOTH") {
		t.Errorf("expected positionSide=BOTH, body=%s", formBody)
	}
	if res.OrderID != "28" {
		t.Errorf("orderID = %q", res.OrderID)
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
