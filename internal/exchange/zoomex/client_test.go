package zoomex

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourname/poscli/internal/config"
	"github.com/yourname/poscli/internal/exchange/bybit"
)

// withTestServer 建一個 Zoomex 風 path prefix 的 httptest server 與 client。
// 直接用 bybit.NewWithBaseURL 即可（這就是 Zoomex.New 做的事），可以避免硬寫
// production URL。Exchange-name override 與 history path 我們在 zoomex.Client 層測試。
func withTestServer(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	creds := &config.Credentials{Name: config.Zoomex, APIKey: []byte("k"), APISecret: []byte("s")}
	inner := bybit.NewWithBaseURL(creds, config.Runtime{HTTPTimeoutSec: 5}, srv.URL, pathPrefix)
	inner.SetName("zoomex")
	return &Client{inner: inner}, srv
}

func TestZoomex_Name(t *testing.T) {
	c, srv := withTestServer(t, func(http.ResponseWriter, *http.Request) {})
	defer srv.Close()
	if c.Name() != "zoomex" {
		t.Fatalf("expected name=zoomex, got %q", c.Name())
	}
}

func TestZoomex_PositionsUsesPathPrefix(t *testing.T) {
	var seenPath string
	c, srv := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathPrefix+"/market/time" {
			_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"timeSecond":"1"},"time":1000}`)
			return
		}
		seenPath = r.URL.Path
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[{"symbol":"BTCUSDT","side":"Buy","size":"1","positionValue":"1","avgPrice":"1","markPrice":"1","leverage":"1","unrealisedPnl":"0","tradeMode":0,"positionIdx":0,"createdTime":"0","updatedTime":"0"}]},"time":1}`)
	})
	defer srv.Close()
	ps, err := c.Positions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(seenPath, pathPrefix+"/position/list") {
		t.Errorf("expected cloud/trade/v3/position/list, got %s", seenPath)
	}
	if len(ps) != 1 || ps[0].Exchange != "zoomex" {
		t.Errorf("expected one position labelled zoomex, got %+v", ps)
	}
}

func TestZoomex_HistoryUsesClosePnlPath(t *testing.T) {
	var seenPath string
	c, srv := withTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == pathPrefix+"/market/time" {
			_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"timeSecond":"1"},"time":1000}`)
			return
		}
		seenPath = r.URL.Path
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[]},"time":1}`)
	})
	defer srv.Close()
	if _, err := c.History(context.Background(), time.Time{}); err != nil {
		t.Fatal(err)
	}
	want := pathPrefix + "/position/close-pnl"
	if seenPath != want {
		t.Errorf("expected %s, got %s", want, seenPath)
	}
}

func TestZoomex_NewProductionURL(t *testing.T) {
	creds := &config.Credentials{Name: config.Zoomex, APIKey: []byte("k"), APISecret: []byte("s")}
	c := New(creds, config.Runtime{HTTPTimeoutSec: 5})
	if c.Name() != "zoomex" {
		t.Fatalf("name = %q", c.Name())
	}
}

func TestZoomex_NewTestnetURL(t *testing.T) {
	// 沒有可直接觀察 baseURL 的方式（bybit.Client 的欄位 unexported），
	// 至少驗證能不出錯地建立、且 Name 正確。Production URL 由其他更高層整合測試確認。
	creds := &config.Credentials{Name: config.Zoomex, APIKey: []byte("k"), APISecret: []byte("s")}
	c := New(creds, config.Runtime{HTTPTimeoutSec: 5, UseTestnet: true})
	if c.Name() != "zoomex" {
		t.Fatalf("name = %q", c.Name())
	}
}
