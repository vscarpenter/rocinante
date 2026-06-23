// Package claudecode is the documented fallback for Claude Code sessions. It
// will watch ~/.claude/projects for recently modified JSONL transcripts and
// tail-parse the last entry for the current task.
//
// The default path is Claude Code hooks, which call "rocinante report" directly
// and need no adapter. This watcher is fragile across versions, so it is the
// backup. Deferred to a later session. See build spec sections 5 and 14.
package claudecode
