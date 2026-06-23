package github

import (
	"context"
	"errors"
	"testing"
)

// oneRepoJSON mirrors the gh pr list shape verified against gh 2.95.0:
// CheckRun rollup elements carry status and conclusion.
const oneRepoJSON = `[
  {"number":1,"reviewDecision":"REVIEW_REQUIRED","isDraft":false,"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"build"}]},
  {"number":2,"reviewDecision":"","isDraft":false,"statusCheckRollup":[{"__typename":"CheckRun","status":"COMPLETED","conclusion":"FAILURE","name":"test"},{"__typename":"CheckRun","status":"COMPLETED","conclusion":"SUCCESS","name":"lint"}]},
  {"number":3,"reviewDecision":"CHANGES_REQUESTED","isDraft":false,"statusCheckRollup":[{"__typename":"CheckRun","status":"IN_PROGRESS","conclusion":"","name":"deploy"}]},
  {"number":4,"reviewDecision":"","isDraft":true,"statusCheckRollup":[]}
]`

func TestParsePRs(t *testing.T) {
	prs, err := parsePRs([]byte(oneRepoJSON))
	if err != nil {
		t.Fatalf("parsePRs: %v", err)
	}
	if len(prs) != 4 {
		t.Fatalf("want 4 PRs, got %d", len(prs))
	}
	if prs[0].Number != 1 || prs[0].ReviewDecision != "REVIEW_REQUIRED" {
		t.Errorf("PR 1 parsed wrong: %+v", prs[0])
	}
	if !prs[3].IsDraft {
		t.Errorf("PR 4 should be a draft")
	}
}

func TestCIState(t *testing.T) {
	cases := []struct {
		name   string
		checks []ghCheck
		want   ciState
	}{
		{"all success is green", []ghCheck{{Typename: "CheckRun", Status: "COMPLETED", Conclusion: "SUCCESS"}}, ciGreen},
		{"success and skipped is green", []ghCheck{{Status: "COMPLETED", Conclusion: "SUCCESS"}, {Status: "COMPLETED", Conclusion: "SKIPPED"}}, ciGreen},
		{"any failure is failing", []ghCheck{{Status: "COMPLETED", Conclusion: "SUCCESS"}, {Status: "COMPLETED", Conclusion: "FAILURE"}}, ciFailing},
		{"in progress is pending", []ghCheck{{Status: "IN_PROGRESS", Conclusion: ""}}, ciPending},
		{"failure beats pending", []ghCheck{{Status: "IN_PROGRESS"}, {Status: "COMPLETED", Conclusion: "FAILURE"}}, ciFailing},
		{"status context error is failing", []ghCheck{{Typename: "StatusContext", State: "ERROR"}}, ciFailing},
		{"status context pending", []ghCheck{{Typename: "StatusContext", State: "PENDING"}}, ciPending},
		{"no checks is none", nil, ciNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ciStateOf(ghPR{StatusCheckRollup: tc.checks}); got != tc.want {
				t.Errorf("ciStateOf = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummarizeRepo(t *testing.T) {
	prs, _ := parsePRs([]byte(oneRepoJSON))
	var c Comms
	c.add(prs)

	if c.OpenPRs != 4 {
		t.Errorf("open PRs: got %d, want 4", c.OpenPRs)
	}
	if c.NeedReview != 2 {
		t.Errorf("need review: got %d, want 2 (drafts and approved excluded)", c.NeedReview)
	}
	if c.CIGreen != 1 || c.CIFailing != 1 || c.CIPending != 1 {
		t.Errorf("CI rollup: green %d failing %d pending %d, want 1/1/1", c.CIGreen, c.CIFailing, c.CIPending)
	}
}

func TestFetchAggregatesAndRecordsRepoErrors(t *testing.T) {
	run := func(_ context.Context, repo string) ([]byte, error) {
		if repo == "owner/bad" {
			return nil, errors.New("could not resolve repo")
		}
		return []byte(oneRepoJSON), nil
	}

	c, err := fetch(context.Background(), []string{"owner/ok1", "owner/ok2", "owner/bad"}, run)
	if err != nil {
		t.Fatalf("partial success should not error: %v", err)
	}
	if c.OpenPRs != 8 {
		t.Errorf("open PRs across two good repos: got %d, want 8", c.OpenPRs)
	}
	if c.NeedReview != 4 {
		t.Errorf("need review: got %d, want 4", c.NeedReview)
	}
	if _, ok := c.Errors["owner/bad"]; !ok {
		t.Errorf("the failing repo should be recorded, errors=%v", c.Errors)
	}
}

func TestFetchTotalFailureReturnsError(t *testing.T) {
	run := func(_ context.Context, _ string) ([]byte, error) {
		return nil, errors.New("gh missing")
	}
	if _, err := fetch(context.Background(), []string{"owner/bad"}, run); err == nil {
		t.Error("when every repo fails, fetch should return an error")
	}
}

func TestParsePRsMalformedErrors(t *testing.T) {
	if _, err := parsePRs([]byte("{not an array")); err == nil {
		t.Error("expected an error on malformed json")
	}
}
