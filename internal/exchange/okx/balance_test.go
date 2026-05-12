package okx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

const balanceJSON = `{
  "code": "0",
  "msg": "",
  "data": [{
    "totalEq": "10362.92",
    "details": [{
      "ccy": "USDT",
      "availBal": "10000.00",
      "availEq": "10050.00",
      "cashBal": "10350.42",
      "eq": "10362.92"
    }, {
      "ccy": "BTC",
      "availBal": "0.01",
      "availEq": "0.01",
      "cashBal": "0.01",
      "eq": "640.0"
    }]
  }]
}`

func TestBalance_FindsUSDTDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, balanceJSON)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	v, err := c.AvailableBalance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if v != 10050 {
		t.Errorf("expected 10050 (availEq), got %v", v)
	}
}
