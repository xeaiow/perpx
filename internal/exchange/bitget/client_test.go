package bitget

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
		Name:       config.Bitget,
		APIKey:     []byte("bg-key"),
		APISecret:  []byte("bg-secret"),
		Passphrase: []byte("bg-passphrase"),
	}
	c := NewWithBaseURL(creds, config.Runtime{HTTPTimeoutSec: 5}, srv.URL)
	c.nowFn = func() time.Time { return time.UnixMilli(1700000000000) }
	c.synced = true
	return c
}

func TestSign_KnownFixture(t *testing.T) {
	ts := "1700000000000"
	got := sign([]byte("bg-secret"), ts, "GET",
		"/api/v2/mix/position/all-position?marginCoin=USDT&productType=USDT-FUTURES", "")
	const want = "mT6PaNFn8uLw/lw60Ze1k1SaZ0Uchabr7KKcGZrkMA8="
	if got != want {
		t.Errorf("sign:\n  got  %s\n  want %s", got, want)
	}
}

func TestDo_GETIncludesQueryInPath(t *testing.T) {
	var seenURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenURL = r.URL.String()
		_, _ = io.WriteString(w, `{"code":"00000","msg":"success","requestTime":1,"data":[]}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	q := map[string][]string{"productType": {"USDT-FUTURES"}, "marginCoin": {"USDT"}}
	if err := c.do(context.Background(), http.MethodGet, "/api/v2/mix/position/all-position", q, nil, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seenURL, "marginCoin=USDT") {
		t.Errorf("URL = %s", seenURL)
	}
}

func TestDo_FiveZeroIsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":"00000","msg":"success","requestTime":1,"data":{"list":[]}}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	var out any
	if err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, &out); err != nil {
		t.Fatal(err)
	}
}

func TestDo_AuthErrMaps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":"40009","msg":"Invalid signature","requestTime":1,"data":null}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, nil)
	if !errorsIs(err, exchange.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

func TestPositions_Happy(t *testing.T) {
	const body = `{
      "code":"00000","msg":"success","requestTime":1,
      "data":[
        {"symbol":"BTCUSDT","marginCoin":"USDT","holdSide":"long","available":"0.1","total":"0.1","leverage":"10","openPriceAvg":"60000","marginMode":"isolated","posMode":"one_way_mode","unrealizedPL":"100","liquidationPrice":"55000","markPrice":"61000","cTime":"0","uTime":"100"},
        {"symbol":"EMPTYUSDT","total":"0","marginCoin":"USDT","markPrice":"0"}
      ]
    }`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.Positions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 (empty filtered), got %d", len(got))
	}
	p := got[0]
	if p.Symbol != "BTCUSDT" || p.Side != exchange.SideLong || p.MarginMode != "isolated" {
		t.Errorf("unexpected: %+v", p)
	}
	if want := 0.1 * 61000.0; p.Notional != want {
		t.Errorf("Notional = %v, want %v", p.Notional, want)
	}
}

func TestClose_PassesHoldSide(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, `{"code":"00000","msg":"success","requestTime":1,"data":{"successList":[{"orderId":"o1","clientOid":""}],"failureList":[]}}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	res, err := c.ClosePosition(context.Background(), exchange.CloseRequest{Symbol: "BTCUSDT", Side: exchange.SideLong})
	if err != nil {
		t.Fatal(err)
	}
	if body["holdSide"] != "long" {
		t.Errorf("holdSide = %v", body["holdSide"])
	}
	if body["productType"] != "USDT-FUTURES" {
		t.Errorf("productType = %v", body["productType"])
	}
	if res.OrderID != "o1" {
		t.Errorf("orderID = %q", res.OrderID)
	}
}

func TestClose_FailureListProducesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":"00000","msg":"success","requestTime":1,"data":{"successList":[],"failureList":[{"orderId":"","errorMsg":"insufficient","errorCode":"E1"}]}}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.ClosePosition(context.Background(), exchange.CloseRequest{Symbol: "BTCUSDT", Side: exchange.SideLong})
	if err == nil {
		t.Fatal("expected error from failureList")
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
