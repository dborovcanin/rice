package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/generation"
	"github.com/dborovcanin/rice/internal/ownership"
)

func newSetupCmd(app func() *App) *cobra.Command {
	var (
		adopt bool
		force bool
	)

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Point application configuration at Rice",
		Long: "Shows what deploying the current generation would do, and with --adopt\n" +
			"carries it out.\n\n" +
			"Setup is a dry run by default and never changes anything without --adopt.\n" +
			"An existing configuration file is copied into backups/ before it is\n" +
			"replaced, and both the original location and its backup are recorded so\n" +
			"`rice uninstall` can reverse the change exactly.\n\n" +
			"A path that is a symlink Rice does not own is reported as a conflict and\n" +
			"left alone; --adopt --force replaces those links too, without touching\n" +
			"whatever they pointed at. A directory standing where a file belongs is\n" +
			"never touched, forced or not.",
		Example: `  # See what would happen. Changes nothing.
  rice setup

  # Do it, backing up anything that exists.
  rice setup --adopt

  # Also take over symlinks pointing outside Rice.
  rice setup --adopt --force

  # Rehearse against a throwaway config directory.
  rice setup --config-dir /tmp/fake-config --adopt`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()

			cfg, err := a.Config()
			if err != nil {
				return err
			}
			if _, err := a.Store.Current(); err != nil {
				if errors.Is(err, generation.ErrNoGenerations) {
					return errors.New("no current generation: run `rice apply` first")
				}
				return err
			}

			plan, err := a.Plan(cfg)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			printPlan(out, plan, a.ConfigDir)

			if !adopt {
				if plan.Empty() && len(plan.Conflicts()) == 0 {
					fmt.Fprintln(out, "\nEverything is already deployed.")
					return nil
				}
				fmt.Fprintln(out, "\nDry run: nothing was changed. Re-run with --adopt to apply.")
				return nil
			}

			if err := ensureNotRoot(); err != nil {
				return err
			}

			result, err := ownership.Apply(plan, a.Paths, ownership.Options{Force: force})
			if err != nil {
				return err
			}
			printResult(out, result)
			return nil
		},
	}

	cmd.Flags().BoolVar(&adopt, "adopt", false, "carry out the plan instead of only showing it")
	cmd.Flags().BoolVar(&force, "force", false, "replace symlinks Rice does not own (implies --adopt)")
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		if force {
			adopt = true
		}
	}
	return cmd
}

// printPlan renders a deployment plan, marking clearly which lines would change
// something and which are refusals.
func printPlan(out io.Writer, plan ownership.Plan, configDir string) {
	fmt.Fprintf(out, "Application configuration under %s\n\n", configDir)

	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, action := range plan.Actions {
		marker := " "
		switch action.Kind {
		case ownership.KindConflict:
			marker = "!"
		case ownership.KindNone:
			marker = "="
		default:
			marker = "+"
		}

		detail := action.Status.State.String()
		if action.Reason != "" {
			detail = action.Reason
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s\n", marker, action.Target, action.Kind, detail)
	}
	w.Flush()

	if adoptions := plan.Adoptions(); len(adoptions) > 0 {
		fmt.Fprintf(out, "\n%d existing file(s) would be backed up before being replaced:\n", len(adoptions))
		for _, a := range adoptions {
			fmt.Fprintf(out, "  %s\n", a.Target)
		}
	}
	if conflicts := plan.Conflicts(); len(conflicts) > 0 {
		fmt.Fprintf(out, "\n%d conflict(s) will be skipped:\n", len(conflicts))
		for _, c := range conflicts {
			hint := ""
			if c.Forceable() {
				hint = " (--force overrides)"
			}
			fmt.Fprintf(out, "  %s: %s%s\n", c.Target, c.Reason, hint)
		}
	}
}

func printResult(out io.Writer, result ownership.Result) {
	fmt.Fprintln(out)
	for _, p := range result.Adopted {
		fmt.Fprintf(out, "adopted  %s\n", p)
	}
	for _, p := range result.Linked {
		fmt.Fprintf(out, "linked   %s\n", p)
	}
	for _, p := range result.Relinked {
		fmt.Fprintf(out, "relinked %s\n", p)
	}
	for _, a := range result.Skipped {
		fmt.Fprintf(out, "skipped  %s: %s\n", a.Target, a.Reason)
	}
	if result.BackupDir != "" {
		fmt.Fprintf(out, "\nOriginals saved in %s\n", result.BackupDir)
	}
	if !result.Changed() {
		fmt.Fprintln(out, "Nothing to do.")
		return
	}
	fmt.Fprintln(out, "\nReload the affected applications with `rice apply`, or log back in.")
}

// ensureNotRoot is a guard for the destructive commands: Rice manages a user's
// own configuration and has no business running as root.
func ensureNotRoot() error {
	if os.Geteuid() == 0 {
		return errors.New("refusing to run as root: Rice manages a user's own configuration")
	}
	return nil
}
