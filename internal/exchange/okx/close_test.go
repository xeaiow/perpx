package okx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourname/poscli/internal/exchange"
)

func TestClose_PassesInstIdAndMgnModePosSide(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, `{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","posSide":"long","clOrdId":""}]}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	res, err := c.ClosePosition(context.Background(), exchange.CloseRequest{
		Symbol: "BTC-USDT-SWAP", Side: exchange.SideLong, MarginMode: "cross",
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["instId"] != "BTC-USDT-SWAP" {
		t.Errorf("instId = %v", body["instId"])
	}
	if body["mgnMode"] != "cross" {
		t.Errorf("mgnMode = %v", body["mgnMode"])
	}
	if body["posSide"] != "long" {
		t.Errorf("posSide = %v", body["posSide"])
	}
	if res.OrderID != "" {
		t.Errorf("OKX close should not return order ID, got %q", res.OrderID)
	}
}
