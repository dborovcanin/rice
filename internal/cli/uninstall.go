package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/ownership"
)

func newUninstallCmd(app func() *App) *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Restore the configuration Rice adopted",
		Long: "Shows how every adoption would be reversed, and with --yes carries it\n" +
			"out.\n\n" +
			"Uninstall is a dry run by default. It reads the adoption manifest, removes\n" +
			"the symlinks Rice installed and copies the backed-up originals back into\n" +
			"place. A path that is no longer the Rice symlink the manifest describes is\n" +
			"left alone: something else owns it now.\n\n" +
			"Nothing else is deleted. Themes, generations, backups and config.toml stay\n" +
			"where they are, so uninstalling and setting up again is cheap.",
		Example: `  # See what would be restored. Changes nothing.
  rice uninstall

  # Do it.
  rice uninstall --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			out := cmd.OutOrStdout()

			manifest, err := a.Manifest()
			if err != nil {
				return err
			}
			if len(manifest.Managed) == 0 {
				fmt.Fprintln(out, "Nothing is adopted; there is nothing to uninstall.")
				return nil
			}

			plan, err := ownership.BuildRestorePlan(manifest, a.Paths)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			for _, action := range plan.Actions {
				switch {
				case action.Skip != "":
					fmt.Fprintf(w, "! %s\tskip\t%s\n", action.Entry.Target, action.Skip)
				case action.Restore != "":
					fmt.Fprintf(w, "+ %s\trestore\tfrom %s\n", action.Entry.Target, action.Entry.Backup)
				default:
					fmt.Fprintf(w, "+ %s\tremove\tnothing was there before\n", action.Entry.Target)
				}
			}
			w.Flush()

			if !confirm {
				fmt.Fprintln(out, "\nDry run: nothing was changed. Re-run with --yes to apply.")
				return nil
			}
			if err := ensureNotRoot(); err != nil {
				return err
			}

			result, err := ownership.Restore(plan, a.Paths)
			if err != nil {
				return err
			}

			fmt.Fprintln(out)
			for _, p := range result.Restored {
				fmt.Fprintf(out, "restored %s\n", p)
			}
			for _, p := range result.Removed {
				fmt.Fprintf(out, "removed  %s\n", p)
			}
			for _, a := range result.Skipped {
				fmt.Fprintf(out, "skipped  %s: %s\n", a.Entry.Target, a.Skip)
			}
			if !result.Changed() {
				fmt.Fprintln(out, "Nothing to do.")
				return nil
			}
			fmt.Fprintf(out, "\nBackups and generations were kept in %s\n", a.Paths.Root)
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "yes", false, "carry out the plan instead of only showing it")
	return cmd
}
