package bybit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yourname/poscli/internal/exchange"
)

const historyJSON = `{
  "retCode": 0,
  "retMsg": "OK",
  "result": {
    "list": [
      {
        "symbol": "BTCUSDT",
        "side": "Sell",
        "qty": "0.15",
        "orderPrice": "64000.0",
        "orderType": "Market",
        "execType": "Trade",
        "closedSize": "0.15",
        "cumEntryValue": "9513.225",
        "avgEntryPrice": "63421.5",
        "cumExitValue": "9601.84",
        "avgExitPrice": "64012.27",
        "closedPnl": "88.61",
        "fillCount": "1",
        "leverage": "10",
        "createdTime": "1684742400000",
        "updatedTime": "1684742410020"
      },
      {
        "symbol": "ETHUSDT",
        "side": "Buy",
        "qty": "2",
        "orderPrice": "3088.0",
        "orderType": "Market",
        "execType": "Trade",
        "closedSize": "2",
        "cumEntryValue": "6240.0",
        "avgEntryPrice": "3120.0",
        "cumExitValue": "6176.0",
        "avgExitPrice": "3088.0",
        "closedPnl": "-64.0",
        "fillCount": "1",
        "leverage": "5",
        "createdTime": "1684700000000",
        "updatedTime": "1684700001000"
      }
    ],
    "nextPageCursor": ""
  },
  "time": 1
}`

func TestHistory_SideFlipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, historyJSON)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	items, err := c.History(context.Background(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	// 第一筆 side=Sell（賣出平倉）→ 原為 long
	if items[0].Side != exchange.SideLong {
		t.Errorf("BTC entry should be long, got %v", items[0].Side)
	}
	// 第二筆 side=Buy（買入平倉）→ 原為 short
	if items[1].Side != exchange.SideShort {
		t.Errorf("ETH entry should be short, got %v", items[1].Side)
	}
	if items[0].RealizedPnL != 88.61 {
		t.Errorf("pnl = %v", items[0].RealizedPnL)
	}
}

func TestHistory_PassesStartTime(t *testing.T) {
	var seenQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[]},"time":1}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	since := time.UnixMilli(1684700000000)
	_, err := c.History(context.Background(), since)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seenQuery, "startTime=1684700000000") {
		t.Errorf("expected startTime in query, got %q", seenQuery)
	}
}

func TestHistory_OmitsStartTimeWhenZero(t *testing.T) {
	var seenQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[]},"time":1}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, _ = c.History(context.Background(), time.Time{})
	if strings.Contains(seenQuery, "startTime=") {
		t.Errorf("expected no startTime when zero, got %q", seenQuery)
	}
}

func TestHistoryAtPath_UsesGivenPath(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[]},"time":1}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, _ = c.HistoryAtPath(context.Background(), "/position/close-pnl", time.Time{})
	if !strings.HasSuffix(seenPath, "/position/close-pnl") {
		t.Errorf("expected close-pnl path, got %q", seenPath)
	}
}
