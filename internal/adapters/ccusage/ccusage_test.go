package ccusage

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// sampleJSON mirrors the shape verified against ccusage 18.0.10:
// daily[] of per-day objects plus a totals object.
const sampleJSON = `{
  "daily": [
    {"date":"2026-06-20","inputTokens":10,"outputTokens":20,"cacheCreationTokens":30,"cacheReadTokens":600,"totalTokens":1000,"totalCost":1.5,"modelsUsed":[],"modelBreakdowns":[]},
    {"date":"2026-06-21","inputTokens":10,"outputTokens":20,"cacheCreationTokens":30,"cacheReadTokens":1600,"totalTokens":2000,"totalCost":2.5,"modelsUsed":[],"modelBreakdowns":[]},
    {"date":"2026-06-22","inputTokens":32776,"outputTokens":174688,"cacheCreationTokens":1151413,"cacheReadTokens":10936657,"totalTokens":12295534,"totalCost":15.52675965,"modelsUsed":[],"modelBreakdowns":[]}
  ],
  "totals": {"inputTokens":32796,"outputTokens":174728,"cacheCreationTokens":1151473,"cacheReadTokens":10938857,"totalCost":19.52,"totalTokens":12298534}
}`

func TestParseUsesMostRecentDay(t *testing.T) {
	r, err := parse([]byte(sampleJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if r.Date != "2026-06-22" {
		t.Errorf("date: got %q, want the most recent day", r.Date)
	}
	if r.TotalTokens != 12295534 {
		t.Errorf("total tokens: got %d", r.TotalTokens)
	}
	if r.CacheReadTokens != 10936657 {
		t.Errorf("cache-read tokens: got %d", r.CacheReadTokens)
	}
	if math.Abs(r.CostUSD-15.52675965) > 1e-9 {
		t.Errorf("cost: got %v", r.CostUSD)
	}
	if got := r.CacheReadRatio(); math.Abs(got-0.8894) > 0.001 {
		t.Errorf("cache-read ratio: got %v, want ~0.889", got)
	}
}

func TestParseBuildsSparklineOldestToNewest(t *testing.T) {
	r, err := parse([]byte(sampleJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []int64{1000, 2000, 12295534}
	if len(r.Spark) != len(want) {
		t.Fatalf("spark length: got %d, want %d", len(r.Spark), len(want))
	}
	for i := range want {
		if r.Spark[i] != want[i] {
			t.Errorf("spark[%d]: got %d, want %d", i, r.Spark[i], want[i])
		}
	}
}

func TestParseEmptyDailyIsZeroNotError(t *testing.T) {
	r, err := parse([]byte(`{"daily":[],"totals":{}}`))
	if err != nil {
		t.Fatalf("empty daily should not error: %v", err)
	}
	if r.TotalTokens != 0 || r.CacheReadRatio() != 0 {
		t.Errorf("empty daily should be zero, got %+v", r)
	}
}

func TestParseMalformedErrors(t *testing.T) {
	if _, err := parse([]byte("{not json")); err == nil {
		t.Error("expected an error on malformed json")
	}
}

func TestFetchRunsCommandAndParses(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "out.json")
	if err := os.WriteFile(file, []byte(sampleJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Fetch(context.Background(), "cat", []string{file})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if r.TotalTokens != 12295534 {
		t.Errorf("fetched total tokens: got %d", r.TotalTokens)
	}
}

func TestFetchFailingCommandErrors(t *testing.T) {
	if _, err := Fetch(context.Background(), "false", nil); err == nil {
		t.Error("a failing command should return an error")
	}
}

func TestFetchHonorsContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := Fetch(ctx, "sleep", []string{"5"}); err == nil {
		t.Error("a slow command should be cut off by the context timeout")
	}
}
