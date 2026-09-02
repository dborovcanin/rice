package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/generation"
)

func newPreviewCmd(app func() *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preview [theme]",
		Short: "Try a theme on the running desktop without committing it",
		Long: "Renders a theme into `preview/`, points `current` at it and reloads, so\n" +
			"the whole desktop changes without a generation being created.\n\n" +
			"Trying six themes should not leave six generations behind. A preview is\n" +
			"never committed, never takes a generation number, and is not what\n" +
			"rollback goes back to. Commit it to build a real generation from the\n" +
			"same theme, or cancel to put `current` back exactly where it was.\n\n" +
			"This is the live preview. The interactive editor's preview is a\n" +
			"different thing: it runs one program against a temporary render and\n" +
			"leaves the desktop alone.",
		Example: `  # Live with a theme for a while.
  rice preview tokyo-night

  # Keep it.
  rice preview commit -m "switched to tokyo night"

  # Or put everything back.
  rice preview cancel

  # What is running?
  rice preview status`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runPreview(cmd, app(), args[0])
		},
		ValidArgsFunction: completeThemes(app),
	}

	cmd.AddCommand(
		newPreviewCommitCmd(app),
		newPreviewCancelCmd(app),
		newPreviewStatusCmd(app),
	)
	return cmd
}

// runPreview renders a theme into the preview directory and switches to it.
func runPreview(cmd *cobra.Command, a *App, themeName string) error {
	cfg, err := a.Config()
	if err != nil {
		return err
	}
	th, err := a.Theme(cfg, themeName)
	if err != nil {
		return err
	}

	state, err := a.Store.Preview(cfg, th)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Previewing %q\n", th.Name)
	if state.Parent != 0 {
		fmt.Fprintf(out, "current -> preview (was generations/%06d)\n", state.Parent)
	} else {
		fmt.Fprintf(out, "current -> preview\n")
	}
	fmt.Fprintln(out, "\n`rice preview commit` keeps it; `rice preview cancel` puts it back.")

	return deployAdopted(cmd, a, cfg, false)
}

func newPreviewCommitCmd(app func() *App) *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Turn the running preview into a generation",
		Long: "Builds an ordinary generation from the theme being previewed and\n" +
			"switches to it, then removes the preview.\n\n" +
			"The generation is built fresh rather than by promoting the preview\n" +
			"directory: rendering is deterministic, and a real build stamps a real\n" +
			"generation number into the files it writes.",
		Example: `  rice preview commit
  rice preview commit -m "warmer palette"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()

			state, err := a.Store.PreviewState()
			if err != nil {
				return err
			}
			cfg, err := a.Config()
			if err != nil {
				return err
			}
			th, err := a.Theme(cfg, state.Theme)
			if err != nil {
				return err
			}

			info, err := a.Store.CommitPreview(cfg, th, generation.BuildOptions{
				Description: description,
			})
			if err != nil {
				return describePreviewError(err)
			}

			// The theme outlives the preview, so it belongs in config.toml.
			if cfg.Theme != th.Name {
				cfg.Theme = th.Name
				if err := writeConfig(a, cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Theme set to %q\n", th.Name)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Generation %06d built from theme %q\n", info.Number, th.Name)
			fmt.Fprintf(out, "current -> generations/%06d\n", info.Number)

			if removed, err := a.Store.Prune(cfg.Generations.Keep); err != nil {
				return err
			} else if len(removed) > 0 {
				fmt.Fprintf(out, "pruned %d old generation(s)\n", len(removed))
			}

			return deployAdopted(cmd, a, cfg, false)
		},
	}

	cmd.Flags().StringVarP(&description, "message", "m", "", "description recorded in the manifest")
	return cmd
}

func newPreviewCancelCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel",
		Short: "Discard the running preview",
		Long: "Points `current` back at the generation it left, removes the preview\n" +
			"and reloads. Nothing about the preview survives.",
		Example: `  rice preview cancel`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()

			state, err := a.Store.CancelPreview()
			if err != nil {
				return describePreviewError(err)
			}

			out := cmd.OutOrStdout()
			if state.Parent != 0 {
				fmt.Fprintf(out, "current -> generations/%06d\n", state.Parent)
			} else {
				fmt.Fprintln(out, "current removed: there was no generation to go back to")
			}

			cfg, err := a.Config()
			if err != nil {
				return err
			}
			return deployAdopted(cmd, a, cfg, false)
		},
	}
}

func newPreviewStatusCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report whether a preview is running",
		Long: "Says whether `current` points at a preview, which theme it renders and\n" +
			"which generation cancelling would return to.",
		Example: `  rice preview status`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			out := cmd.OutOrStdout()

			if !a.Store.PreviewActive() {
				fmt.Fprintln(out, "No preview is active.")
				return nil
			}

			state, err := a.Store.PreviewState()
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Previewing %q\n", state.Theme)
			if state.Parent != 0 {
				fmt.Fprintf(out, "cancel returns to generations/%06d\n", state.Parent)
			} else {
				fmt.Fprintln(out, "cancel has no generation to return to")
			}
			return nil
		},
	}
}

// describePreviewError turns the store's sentinels into advice.
func describePreviewError(err error) error {
	if errors.Is(err, generation.ErrNoPreview) {
		return errors.New("no preview is active: start one with `rice preview <theme>`")
	}
	return err
}

// requireNoPreview refuses an operation that would fight with a running
// preview. Applying or rolling back underneath one would move `current` away
// and leave the preview state describing something that is no longer true.
func requireNoPreview(a *App, action string) error {
	if !a.Store.PreviewActive() {
		return nil
	}
	return fmt.Errorf(
		"a preview is active: %s would discard it silently.\n"+
			"Run `rice preview commit` to keep it, or `rice preview cancel` to drop it",
		action,
	)
}
