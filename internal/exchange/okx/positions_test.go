package okx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourname/poscli/internal/exchange"
)

const positionsJSON = `{
  "code": "0",
  "msg": "",
  "data": [
    {
      "instId": "BTC-USDT-SWAP",
      "instType": "SWAP",
      "mgnMode": "cross",
      "posSide": "long",
      "pos": "0.15",
      "avgPx": "63421.5",
      "markPx": "64012.3",
      "upl": "88.62",
      "lever": "10",
      "notionalUsd": "9601.84",
      "ccy": "USDT",
      "uTime": "1720736417660",
      "cTime": "1720700000000"
    },
    {
      "instId": "ETH-USDT-SWAP",
      "instType": "SWAP",
      "mgnMode": "isolated",
      "posSide": "net",
      "pos": "-2.5",
      "avgPx": "3120",
      "markPx": "3088",
      "upl": "80",
      "lever": "5",
      "notionalUsd": "7720",
      "ccy": "USDT",
      "uTime": "1720736417661",
      "cTime": "1720700000000"
    },
    {
      "instId": "BTC-USD-SWAP",
      "instType": "SWAP",
      "mgnMode": "cross",
      "posSide": "long",
      "pos": "1",
      "ccy": "BTC",
      "uTime": "0",
      "cTime": "0"
    },
    {
      "instId": "EMPTY-USDT-SWAP",
      "instType": "SWAP",
      "pos": "0",
      "ccy": "USDT"
    }
  ]
}`

func TestPositions_Happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, positionsJSON)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.Positions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 positions (inverse + empty filtered), got %d:\n%+v", len(got), got)
	}
	var btc, eth *exchange.Position
	for i := range got {
		switch got[i].Symbol {
		case "BTCUSDT":
			btc = &got[i]
		case "ETHUSDT":
			eth = &got[i]
		}
	}
	if btc == nil || eth == nil {
		t.Fatalf("missing entries: %+v", got)
	}
	if btc.RawSymbol != "BTC-USDT-SWAP" {
		t.Errorf("raw symbol = %q", btc.RawSymbol)
	}
	if eth.Side != exchange.SideShort {
		t.Errorf("ETH should be short via net mode, got %v", eth.Side)
	}
	if eth.Size != 2.5 {
		t.Errorf("ETH size should be |-2.5|, got %v", eth.Size)
	}
	if btc.MarginMode != "cross" {
		t.Errorf("margin = %q", btc.MarginMode)
	}
	if btc.Exchange != "okx" {
		t.Errorf("exchange = %q", btc.Exchange)
	}
}
