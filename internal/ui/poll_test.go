package ui

import (
	"strings"
	"testing"

	"github.com/vscarpenter/rocinante/internal/adapters/ccusage"
	"github.com/vscarpenter/rocinante/internal/adapters/github"
)

func TestReactorMsgRendersBurn(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	r := ccusage.Reactor{
		TotalTokens: 12295534, CacheReadTokens: 10936657,
		CostUSD: 15.53, Spark: []int64{1000, 2000, 12295534},
	}
	next, _ := m.Update(reactorMsg(r))
	m2 := next.(model)

	if m2.reactor == nil {
		t.Fatal("reactor result was not stored")
	}
	view := m2.View()
	if !strings.Contains(view, "12.3M tok") {
		t.Errorf("expected the token headline in the view:\n%s", view)
	}
	if !strings.Contains(view, "cache-read 89%") {
		t.Errorf("expected the cache-read ratio in the view:\n%s", view)
	}
}

func TestCommsMsgRendersSummary(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	c := github.Comms{OpenPRs: 3, NeedReview: 2, CIGreen: 3, Errors: map[string]string{}}
	next, _ := m.Update(commsMsg(c))
	view := next.(model).View()

	for _, want := range []string{"3 open PRs", "2 need review", "CI green"} {
		if !strings.Contains(view, want) {
			t.Errorf("comms view missing %q:\n%s", want, view)
		}
	}
}

func TestErrMsgSurfacesAdapterError(t *testing.T) {
	m := sizedModel(t0, freshAgent("a", "A", t0))
	next, _ := m.Update(errMsg{source: "reactor", text: "ccusage exploded"})
	m2 := next.(model)

	if m2.reactorErr == "" {
		t.Fatal("reactor error was not recorded")
	}
	if !strings.Contains(m2.View(), "ccusage exploded") {
		t.Errorf("the adapter error should surface in the panel:\n%s", m2.View())
	}
}

func TestRefreshKeyIssuesFetch(t *testing.T) {
	m := sizedModel(t0)
	_, cmd := m.Update(key("r"))
	if cmd == nil {
		t.Error("r should issue a refresh command when adapters are enabled")
	}
}
