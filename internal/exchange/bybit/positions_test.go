package bybit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourname/poscli/internal/exchange"
)

const positionsHappyJSON = `{
  "retCode": 0,
  "retMsg": "OK",
  "result": {
    "list": [
      {
        "symbol": "BTCUSDT",
        "side": "Buy",
        "size": "0.150",
        "positionValue": "9601.84",
        "avgPrice": "63421.5",
        "markPrice": "64012.3",
        "leverage": "10",
        "unrealisedPnl": "88.62",
        "tradeMode": 0,
        "positionIdx": 0,
        "createdTime": "1676538056258",
        "updatedTime": "1684742400015"
      },
      {
        "symbol": "ETHUSDT",
        "side": "Sell",
        "size": "2.5",
        "positionValue": "7720.0",
        "avgPrice": "3120.0",
        "markPrice": "3088.0",
        "leverage": "5",
        "unrealisedPnl": "80.0",
        "tradeMode": 1,
        "positionIdx": 0,
        "createdTime": "0",
        "updatedTime": "1684742400016"
      },
      {
        "symbol": "DOGEUSDT",
        "side": "",
        "size": "0",
        "positionValue": "0",
        "avgPrice": "0",
        "markPrice": "0",
        "leverage": "10",
        "unrealisedPnl": "0",
        "tradeMode": 0,
        "positionIdx": 0,
        "createdTime": "0",
        "updatedTime": "0"
      }
    ],
    "category": "linear",
    "nextPageCursor": ""
  },
  "time": 1684742400000
}`

func TestPositions_HappyAndNormalize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, positionsHappyJSON)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.Positions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 positions (1 empty filtered), got %d", len(got))
	}
	// BTCUSDT 應為 long、size 0.150；ETHUSDT 為 short、size 2.5。
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
	if btc.Side != exchange.SideLong {
		t.Errorf("BTC side = %v", btc.Side)
	}
	if btc.Size != 0.150 {
		t.Errorf("BTC size = %v", btc.Size)
	}
	if btc.UnrealizedPnL != 88.62 {
		t.Errorf("BTC uPnL = %v", btc.UnrealizedPnL)
	}
	if btc.MarginMode != "cross" {
		t.Errorf("BTC margin = %q", btc.MarginMode)
	}
	if btc.Exchange != "bybit" {
		t.Errorf("BTC exchange = %q", btc.Exchange)
	}
	if eth.Side != exchange.SideShort {
		t.Errorf("ETH side = %v", eth.Side)
	}
	if eth.MarginMode != "isolated" {
		t.Errorf("ETH margin = %q", eth.MarginMode)
	}
}

func TestPositions_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"retCode":10003,"retMsg":"Invalid API key","result":null}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.Positions(context.Background())
	if !errorsIs(err, exchange.ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}
