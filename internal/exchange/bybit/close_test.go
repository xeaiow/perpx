package bybit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourname/poscli/internal/exchange"
)

func TestClose_LongSendsSell(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"orderId":"ord-1","orderLinkId":""},"time":1}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	res, err := c.ClosePosition(context.Background(), exchange.CloseRequest{
		Symbol: "BTCUSDT",
		Side:   exchange.SideLong,
		Size:   0.15,
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["side"] != "Sell" {
		t.Errorf("expected Sell for long close, got %v", body["side"])
	}
	if body["reduceOnly"] != true {
		t.Errorf("expected reduceOnly=true, got %v", body["reduceOnly"])
	}
	if body["orderType"] != "Market" {
		t.Errorf("expected orderType=Market, got %v", body["orderType"])
	}
	if body["category"] != "linear" {
		t.Errorf("expected category=linear, got %v", body["category"])
	}
	if body["qty"] != "0.15" {
		t.Errorf("expected qty=0.15 string, got %v", body["qty"])
	}
	if res.OrderID != "ord-1" {
		t.Errorf("orderID = %q", res.OrderID)
	}
}

func TestClose_ShortSendsBuy(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"orderId":"ord-2"},"time":1}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.ClosePosition(context.Background(), exchange.CloseRequest{
		Symbol: "ETHUSDT",
		Side:   exchange.SideShort,
		Size:   2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if body["side"] != "Buy" {
		t.Errorf("expected Buy for short close, got %v", body["side"])
	}
}

func TestClose_OmitsQtyWhenZero(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		_, _ = io.WriteString(w, `{"retCode":0,"retMsg":"OK","result":{"orderId":"ord-3"},"time":1}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, _ = c.ClosePosition(context.Background(), exchange.CloseRequest{
		Symbol: "BTCUSDT",
		Side:   exchange.SideLong,
		Size:   0, // 0 → 全部
	})
	if _, ok := body["qty"]; ok {
		t.Errorf("expected qty omitted when size=0, body=%s", mustJSONBytes(body))
	}
}

func mustJSONBytes(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
