package ui

import (
	"testing"
	"unicode/utf8"
)

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1500, "1.5K"},
		{12295534, "12.3M"},
		{1910000000, "1.91B"},
	}
	for _, tc := range cases {
		if got := formatTokens(tc.n); got != tc.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestSparklineEmpty(t *testing.T) {
	if got := sparkline(nil); got != "" {
		t.Errorf("empty sparkline should be empty, got %q", got)
	}
}

func TestSparklineMapsRange(t *testing.T) {
	got := sparkline([]int64{0, 50, 100})
	if n := utf8.RuneCountInString(got); n != 3 {
		t.Fatalf("sparkline length: got %d runes, want 3 (%q)", n, got)
	}
	runes := []rune(got)
	if runes[0] != '▁' {
		t.Errorf("lowest value should be the lowest tick, got %q", string(runes[0]))
	}
	if runes[2] != '█' {
		t.Errorf("highest value should be the highest tick, got %q", string(runes[2]))
	}
}

func TestSparklineFlatDataDoesNotPanic(t *testing.T) {
	if got := sparkline([]int64{5, 5, 5}); utf8.RuneCountInString(got) != 3 {
		t.Errorf("flat data should still render one tick per sample, got %q", got)
	}
}
