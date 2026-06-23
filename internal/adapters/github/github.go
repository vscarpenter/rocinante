package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// prLimit caps how many open PRs we read per repo.
const prLimit = "50"

// Comms is the parsed GitHub snapshot the panel renders, aggregated across all
// watched repos. Errors holds the repos that failed, so one bad repo degrades to
// a line instead of taking down the panel.
type Comms struct {
	OpenPRs    int
	NeedReview int
	CIGreen    int
	CIFailing  int
	CIPending  int
	Errors     map[string]string
}

// ghPR is one pull request from gh pr list. Only the fields the rollup needs are
// decoded.
type ghPR struct {
	Number            int       `json:"number"`
	Title             string    `json:"title"`
	ReviewDecision    string    `json:"reviewDecision"`
	IsDraft           bool      `json:"isDraft"`
	StatusCheckRollup []ghCheck `json:"statusCheckRollup"`
}

// ghCheck is one entry in a PR's statusCheckRollup. CheckRun entries carry a
// status and conclusion; legacy StatusContext entries carry a state.
type ghCheck struct {
	Typename   string `json:"__typename"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

// ciState is a PR's rolled-up check status.
type ciState string

const (
	ciGreen   ciState = "green"
	ciFailing ciState = "failing"
	ciPending ciState = "pending"
	ciNone    ciState = "none"
)

// Fetch reads open PRs for each repo and aggregates them. It returns an error
// only when every repo fails, so partial results still reach the panel.
func Fetch(ctx context.Context, repos []string) (Comms, error) {
	return fetch(ctx, repos, ghRunner)
}

// runner reads the raw gh pr list JSON for one repo. It is a seam so the
// aggregation can be tested without the gh binary.
type runner func(ctx context.Context, repo string) ([]byte, error)

func fetch(ctx context.Context, repos []string, run runner) (Comms, error) {
	c := Comms{Errors: map[string]string{}}
	var ok int

	for _, repo := range repos {
		data, err := run(ctx, repo)
		if err != nil {
			c.Errors[repo] = err.Error()
			continue
		}
		prs, err := parsePRs(data)
		if err != nil {
			c.Errors[repo] = err.Error()
			continue
		}
		c.add(prs)
		ok++
	}

	if ok == 0 && len(repos) > 0 {
		return c, fmt.Errorf("github: every watched repo failed")
	}
	return c, nil
}

// ghRunner runs gh pr list for one repo under ctx.
func ghRunner(ctx context.Context, repo string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "list",
		"--repo", repo, "--state", "open", "--limit", prLimit,
		"--json", "number,title,reviewDecision,statusCheckRollup,isDraft")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

// parsePRs decodes one repo's gh pr list output.
func parsePRs(data []byte) ([]ghPR, error) {
	var prs []ghPR
	if err := json.Unmarshal(data, &prs); err != nil {
		return nil, fmt.Errorf("github: parse json: %w", err)
	}
	return prs, nil
}

// add folds one repo's PRs into the aggregate.
func (c *Comms) add(prs []ghPR) {
	for _, pr := range prs {
		c.OpenPRs++
		if !pr.IsDraft && needsReview(pr.ReviewDecision) {
			c.NeedReview++
		}
		switch ciStateOf(pr) {
		case ciGreen:
			c.CIGreen++
		case ciFailing:
			c.CIFailing++
		case ciPending:
			c.CIPending++
		}
	}
}

// needsReview reports whether a review decision still wants attention.
func needsReview(decision string) bool {
	return decision == "REVIEW_REQUIRED" || decision == "CHANGES_REQUESTED"
}

// ciStateOf rolls a PR's checks into one state. Failing beats pending, which
// beats green. A PR with no checks is none.
func ciStateOf(pr ghPR) ciState {
	var sawFailing, sawPending, sawAny bool
	for _, ch := range pr.StatusCheckRollup {
		sawAny = true
		switch {
		case isFailing(ch):
			sawFailing = true
		case isPending(ch):
			sawPending = true
		}
	}
	switch {
	case sawFailing:
		return ciFailing
	case sawPending:
		return ciPending
	case sawAny:
		return ciGreen
	default:
		return ciNone
	}
}

func isFailing(ch ghCheck) bool {
	switch ch.Conclusion {
	case "FAILURE", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "STARTUP_FAILURE":
		return true
	}
	switch ch.State {
	case "FAILURE", "ERROR":
		return true
	}
	return false
}

func isPending(ch ghCheck) bool {
	// A CheckRun that has not completed is pending.
	if ch.Status != "" && ch.Status != "COMPLETED" {
		return true
	}
	switch ch.State {
	case "PENDING", "EXPECTED":
		return true
	}
	return false
}
