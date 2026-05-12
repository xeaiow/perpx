package bybit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const balanceUnifiedJSON = `{
  "retCode": 0,
  "retMsg": "OK",
  "result": {
    "list": [{
      "accountType": "UNIFIED",
      "coin": [{
        "coin": "USDT",
        "walletBalance": "10350.42",
        "availableToWithdraw": "10000.00",
        "unrealisedPnl": "12.50",
        "equity": "10362.92"
      }, {
        "coin": "BNB",
        "walletBalance": "1.0",
        "availableToWithdraw": "1.0",
        "unrealisedPnl": "0",
        "equity": "1.0"
      }]
    }]
  },
  "time": 1
}`

func TestBalance_UnifiedHappy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, balanceUnifiedJSON)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	v, err := c.AvailableBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != 10000.00 {
		t.Errorf("expected 10000.00, got %v", v)
	}
}

func TestBalance_FallbackToContract(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.RawQuery, "accountType=UNIFIED") {
			// 模擬 unified 不支援
			_, _ = io.WriteString(w, `{"retCode":30086,"retMsg":"accountType not supported","result":null}`)
			return
		}
		// CONTRACT 走得通
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[{"accountType":"CONTRACT","coin":[{"coin":"USDT","walletBalance":"500","availableToWithdraw":"450","unrealisedPnl":"0","equity":"500"}]}]},"time":1}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	v, err := c.AvailableBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != 450 {
		t.Errorf("expected 450 from CONTRACT fallback, got %v", v)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (unified+contract), got %d", calls)
	}
}

func TestBalance_EmptyListReturnsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"list":[]},"time":1}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	v, err := c.AvailableBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Errorf("expected 0, got %v", v)
	}
}
