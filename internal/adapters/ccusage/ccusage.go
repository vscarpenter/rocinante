package ccusage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// sparkWindow caps how many recent days feed the sparkline.
const sparkWindow = 14

// Reactor is the parsed token-burn snapshot the panel renders. It is the
// adapter's only output type, so the UI never sees ccusage's raw shape.
type Reactor struct {
	Date            string
	TotalTokens     int64
	CacheReadTokens int64
	CostUSD         float64
	Spark           []int64 // recent daily totals, oldest to newest
}

// CacheReadRatio is the share of today's tokens served from cache, in [0,1]. It
// returns zero when there are no tokens, so the panel never divides by zero.
func (r Reactor) CacheReadRatio() float64 {
	if r.TotalTokens == 0 {
		return 0
	}
	return float64(r.CacheReadTokens) / float64(r.TotalTokens)
}

// ccDay is one day in ccusage's daily array. Only the fields the Reactor needs
// are decoded.
type ccDay struct {
	Date            string  `json:"date"`
	TotalTokens     int64   `json:"totalTokens"`
	CacheReadTokens int64   `json:"cacheReadTokens"`
	TotalCost       float64 `json:"totalCost"`
}

type ccOutput struct {
	Daily []ccDay `json:"daily"`
}

// Fetch runs the ccusage command under ctx and parses its JSON. The caller sets
// the timeout on ctx, so a hung ccusage never stalls the bridge.
func Fetch(ctx context.Context, command string, args []string) (Reactor, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return Reactor{}, fmt.Errorf("ccusage: %w: %s", err, msg)
		}
		return Reactor{}, fmt.Errorf("ccusage: %w", err)
	}
	return parse(stdout.Bytes())
}

// parse turns ccusage daily JSON into a Reactor. The most recent day is the
// headline, and the trailing window feeds the sparkline. An empty daily array
// is valid and means no usage, not an error.
func parse(data []byte) (Reactor, error) {
	var out ccOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return Reactor{}, fmt.Errorf("ccusage: parse json: %w", err)
	}
	if len(out.Daily) == 0 {
		return Reactor{}, nil
	}

	last := out.Daily[len(out.Daily)-1]
	r := Reactor{
		Date:            last.Date,
		TotalTokens:     last.TotalTokens,
		CacheReadTokens: last.CacheReadTokens,
		CostUSD:         last.TotalCost,
	}

	start := 0
	if len(out.Daily) > sparkWindow {
		start = len(out.Daily) - sparkWindow
	}
	for _, d := range out.Daily[start:] {
		r.Spark = append(r.Spark, d.TotalTokens)
	}
	return r, nil
}
