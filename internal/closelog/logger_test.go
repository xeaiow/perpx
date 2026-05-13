package closelog

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTempLogger(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "close.log")
	if err := InitWithPath(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func readLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	Sync()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		out = append(out, m)
	}
	return out
}

func sampleFields() Fields {
	return Fields{
		Exchange:   "okx",
		Symbol:     "AIUSDT",
		RawSymbol:  "AI-USDT-SWAP",
		Side:       "long",
		Size:       3500,
		CoinSize:   35,
		EntryPrice: 0.02308,
		MarkPrice:  0.02341,
		UPnL:       1.155,
		MarginMode: "cross",
	}
}

func TestRequestedCompletedRoundtrip(t *testing.T) {
	path := newTempLogger(t)
	f := sampleFields()
	Requested(f)
	Completed(f, "ord-xyz", 250*time.Millisecond)

	lines := readLines(t, path)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0]["msg"] != "close.requested" {
		t.Errorf("first msg = %v", lines[0]["msg"])
	}
	if lines[1]["msg"] != "close.completed" {
		t.Errorf("second msg = %v", lines[1]["msg"])
	}
	if lines[1]["orderID"] != "ord-xyz" {
		t.Errorf("orderID = %v", lines[1]["orderID"])
	}
	if lines[1]["latencyMs"].(float64) != 250 {
		t.Errorf("latencyMs = %v", lines[1]["latencyMs"])
	}
	if lines[0]["exchange"] != "okx" || lines[0]["symbol"] != "AIUSDT" {
		t.Errorf("base fields missing: %+v", lines[0])
	}
}

func TestFailedRecordsError(t *testing.T) {
	path := newTempLogger(t)
	Failed(sampleFields(), errors.New("boom"), 80*time.Millisecond)
	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0]["error"] != "boom" {
		t.Errorf("error field = %v", lines[0]["error"])
	}
}

func TestCancelledMinimal(t *testing.T) {
	path := newTempLogger(t)
	Cancelled(sampleFields())
	lines := readLines(t, path)
	if len(lines) != 1 || lines[0]["msg"] != "close.cancelled" {
		t.Fatalf("unexpected: %+v", lines)
	}
}
