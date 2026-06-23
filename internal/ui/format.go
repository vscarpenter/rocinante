package ui

import (
	"fmt"
	"math"
)

// formatTokens renders a token count compactly, such as 999, 1.5K, 12.3M, or
// 1.91B.
func formatTokens(n int64) string {
	f := float64(n)
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.2fB", f/1_000_000_000)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", f/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", f/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// sparkTicks are the eight block heights used for the sparkline.
var sparkTicks = []rune("▁▂▃▄▅▆▇█")

// sparkline renders samples as a row of block characters scaled between the
// smallest and largest value. Flat data renders at the lowest tick, and an
// empty slice renders nothing.
func sparkline(samples []int64) string {
	if len(samples) == 0 {
		return ""
	}
	min, max := samples[0], samples[0]
	for _, v := range samples {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	out := make([]rune, len(samples))
	span := float64(max - min)
	for i, v := range samples {
		level := 0
		if span > 0 {
			level = int(math.Round(float64(v-min) / span * float64(len(sparkTicks)-1)))
		}
		out[i] = sparkTicks[level]
	}
	return string(out)
}
