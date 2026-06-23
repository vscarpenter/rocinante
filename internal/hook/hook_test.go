package hook

import (
	"strings"
	"testing"

	"github.com/vscarpenter/rocinante/internal/fleet"
)

func TestOptionsForSessionStart(t *testing.T) {
	p := Payload{SessionID: "abc123", Cwd: "/Users/vinny/dev/rocinante", Event: "SessionStart", Source: "startup"}
	opts, ok := optionsFor(p)
	if !ok {
		t.Fatal("SessionStart should produce a report")
	}
	if opts.ID != "cc-abc123" {
		t.Errorf("id: got %q, want cc-abc123", opts.ID)
	}
	if opts.Name != "rocinante" {
		t.Errorf("name should be the cwd basename, got %q", opts.Name)
	}
	if opts.Kind != string(fleet.KindClaudeCode) {
		t.Errorf("kind: got %q", opts.Kind)
	}
	if opts.State != "running" {
		t.Errorf("state: got %q, want running", opts.State)
	}
	if !strings.Contains(opts.Task, "started") {
		t.Errorf("task should mention started, got %q", opts.Task)
	}
	for _, f := range []string{"name", "kind", "state", "task"} {
		if !opts.Provided[f] {
			t.Errorf("field %q should be marked provided", f)
		}
	}
}

func TestOptionsForUserPromptSubmitUsesFirstLine(t *testing.T) {
	p := Payload{SessionID: "x", Cwd: "/tmp/proj", Event: "UserPromptSubmit", Prompt: "Fix the render bug\nand also tidy up"}
	opts, ok := optionsFor(p)
	if !ok {
		t.Fatal("UserPromptSubmit should produce a report")
	}
	if opts.State != "running" {
		t.Errorf("state: got %q", opts.State)
	}
	if opts.Task != "Fix the render bug" {
		t.Errorf("task should be the first line of the prompt, got %q", opts.Task)
	}
}

func TestOptionsForPreToolUseIsHeartbeatOnly(t *testing.T) {
	p := Payload{SessionID: "x", Cwd: "/tmp/proj", Event: "PreToolUse", ToolName: "Bash"}
	opts, ok := optionsFor(p)
	if !ok {
		t.Fatal("PreToolUse should refresh the heartbeat")
	}
	if opts.State != "running" {
		t.Errorf("state: got %q, want running", opts.State)
	}
	// It must not overwrite the task or detail, so the prompt-derived task and
	// the Ship's Log stay quiet during a busy turn.
	if opts.Provided["task"] || opts.Provided["detail"] {
		t.Errorf("PreToolUse should not set task or detail, provided=%v", opts.Provided)
	}
}

func TestOptionsForStopAndEnd(t *testing.T) {
	stop, ok := optionsFor(Payload{SessionID: "x", Cwd: "/tmp/p", Event: "Stop"})
	if !ok || stop.State != "idle" {
		t.Errorf("Stop should set idle, got %q ok=%v", stop.State, ok)
	}
	end, ok := optionsFor(Payload{SessionID: "x", Cwd: "/tmp/p", Event: "SessionEnd"})
	if !ok || end.State != "offline" {
		t.Errorf("SessionEnd should set offline, got %q ok=%v", end.State, ok)
	}
}

func TestOptionsForIgnoredEvents(t *testing.T) {
	if _, ok := optionsFor(Payload{SessionID: "x", Event: "Notification"}); ok {
		t.Error("an unhandled event should be ignored")
	}
	if _, ok := optionsFor(Payload{SessionID: "", Event: "SessionStart"}); ok {
		t.Error("a missing session id should be ignored")
	}
}

func TestRunWritesAndMerges(t *testing.T) {
	dir := t.TempDir()

	start := `{"session_id":"s1","cwd":"/Users/vinny/dev/inkwell","hook_event_name":"SessionStart","source":"startup"}`
	if err := Run(dir, strings.NewReader(start)); err != nil {
		t.Fatalf("Run SessionStart: %v", err)
	}
	s, err := fleet.Read(fleet.Path(dir, "cc-s1"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if s.Name != "inkwell" || s.Kind != fleet.KindClaudeCode || s.State != fleet.StateRunning {
		t.Errorf("session start file wrong: %+v", s)
	}

	prompt := `{"session_id":"s1","cwd":"/Users/vinny/dev/inkwell","hook_event_name":"UserPromptSubmit","prompt":"add a dark theme"}`
	if err := Run(dir, strings.NewReader(prompt)); err != nil {
		t.Fatalf("Run UserPromptSubmit: %v", err)
	}
	s, _ = fleet.Read(fleet.Path(dir, "cc-s1"))
	if s.Task != "add a dark theme" {
		t.Errorf("prompt should become the task, got %q", s.Task)
	}
	if s.Name != "inkwell" {
		t.Errorf("merge should preserve the name, got %q", s.Name)
	}
}

func TestRunIgnoresUnknownEvent(t *testing.T) {
	dir := t.TempDir()
	body := `{"session_id":"s9","cwd":"/tmp/p","hook_event_name":"Notification"}`
	if err := Run(dir, strings.NewReader(body)); err != nil {
		t.Fatalf("Run should not error on an ignored event: %v", err)
	}
	if _, err := fleet.Read(fleet.Path(dir, "cc-s9")); err == nil {
		t.Error("an ignored event should not write a file")
	}
}

func TestRunMalformedPayloadErrors(t *testing.T) {
	if err := Run(t.TempDir(), strings.NewReader("{not json")); err == nil {
		t.Error("malformed payload should return an error")
	}
}
