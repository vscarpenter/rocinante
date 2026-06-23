// Command rocinante is a terminal cockpit for your agent fleet.
//
// Invoked bare, it launches the TUI bridge. Invoked as "rocinante report",
// it writes or updates an agent status file, then exits. One binary, two
// modes, so any agent, hook, or cron job can announce itself without knowing
// the file format, the directory, or the schema version.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/vscarpenter/rocinante/internal/config"
	"github.com/vscarpenter/rocinante/internal/hook"
	"github.com/vscarpenter/rocinante/internal/report"
	"github.com/vscarpenter/rocinante/internal/ui"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "rocinante:", err)
		os.Exit(1)
	}
}

// newRootCmd builds the command tree. The bare command launches the TUI, and
// report is a subcommand of the same binary.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "rocinante",
		Short:         "A terminal cockpit for your agent fleet",
		Long:          "Rocinante is the bridge you watch your whole agent fleet from. It reads status that agents already report; it does not orchestrate them.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			return ui.Run(cfg)
		},
	}
	root.AddCommand(newReportCmd())
	root.AddCommand(newHookCmd())
	return root
}

// newHookCmd wires the hook subcommand. Claude Code runs it for a lifecycle
// event and passes the event JSON on stdin. It never fails loudly: any error is
// logged to stderr but the command exits zero, so a status hook can never
// disrupt the session it is observing.
func newHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "hook",
		Short:  "Translate a Claude Code hook event on stdin into a status report",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				fmt.Fprintln(os.Stderr, "rocinante hook:", err)
				return nil
			}
			if err := hook.Run(cfg.Fleet.Dir, cmd.InOrStdin()); err != nil {
				fmt.Fprintln(os.Stderr, "rocinante hook:", err)
			}
			return nil
		},
	}
}

// newReportCmd wires the report subcommand and its flags. It binds each flag
// to an Options field, then records which flags the caller actually set so the
// report logic can merge rather than clobber unspecified fields.
func newReportCmd() *cobra.Command {
	var opts report.Options
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Write or update an agent status file, then exit",
		Long:  "report writes one status file per agent under the fleet directory, using an atomic temp-file-plus-rename so the bridge never reads a half-written file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			opts.Provided = changedFlags(cmd, "name", "kind", "state", "task", "detail", "tokens")
			return report.Run(cfg.Fleet.Dir, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.ID, "id", "", "stable, unique agent id (required); becomes the filename")
	f.StringVar(&opts.Name, "name", "", "human label shown in the Fleet panel")
	f.StringVar(&opts.Kind, "kind", "", "agent kind: always-on, cron, launchd, claude-code, other")
	f.StringVar(&opts.State, "state", "", "agent state: running, idle, blocked, error, offline")
	f.StringVar(&opts.Task, "task", "", "one line describing current work")
	f.StringVar(&opts.Detail, "detail", "", "longer text or last action for the inspect view")
	f.Int64Var(&opts.Tokens, "tokens", 0, "tokens consumed today")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}

// changedFlags returns the set of named flags the caller explicitly set. The
// report logic uses it to merge only provided fields into an existing file.
func changedFlags(cmd *cobra.Command, names ...string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		if cmd.Flags().Changed(n) {
			set[n] = true
		}
	}
	return set
}
