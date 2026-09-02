package cli

import (
	"github.com/spf13/cobra"

	rice "github.com/dborovcanin/rice"
)

// Execute runs the Rice command tree.
func Execute() error {
	return NewRootCmd().Execute()
}

// NewRootCmd builds the command tree. The Rice root is resolved lazily in
// PersistentPreRunE so --root applies to every subcommand.
func NewRootCmd() *cobra.Command {
	var (
		root      string
		configDir string
		app       *App
	)

	cmd := &cobra.Command{
		Use:     "rice",
		Short:   "Generate consistent configuration for a SwayFX desktop",
		Version: rice.Version,
		Long: "Rice generates complete configuration files for SwayFX, Waybar, Rofi,\n" +
			"Foot, Dunst and swaylock from one theme and one source configuration.\n\n" +
			"Appearance lives in a theme; structure lives in config.toml. Each change\n" +
			"produces an immutable generation, and `current` selects the active one.\n\n" +
			"Run with no arguments on a terminal to open the interactive editor.",
		Example: `  # Pick and edit a theme interactively.
  rice

  # First run.
  rice init
  rice apply

  # Switch theme.
  rice theme list
  rice theme apply tokyo-night

  # Undo it.
  rice rollback

  # Try everything against a throwaway root.
  rice --root /tmp/rice-test init
  rice --root /tmp/rice-test apply`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			a, err := NewApp(root, configDir)
			if err != nil {
				return err
			}
			app = a
			return nil
		},
		// Bare `rice` opens the editor when there is a terminal to draw on,
		// and prints help otherwise, so piping it still behaves like a CLI.
		RunE: func(cmd *cobra.Command, args []string) error {
			if !interactive() {
				return cmd.Help()
			}
			return runTUI(cmd, app, "")
		},
	}

	cmd.PersistentFlags().StringVar(&root, "root", "",
		"Rice root directory (default $RICE_HOME or ~/.config/rice)")
	cmd.PersistentFlags().StringVar(&configDir, "config-dir", "",
		"where applications keep their configuration (default $XDG_CONFIG_HOME or ~/.config)")

	get := func() *App { return app }

	cmd.AddCommand(
		newTUICmd(get),
		newInitCmd(get),
		newApplyCmd(get),
		newRenderCmd(get),
		newDiffCmd(get),
		newPreviewCmd(get),
		newRollbackCmd(get),
		newThemeCmd(get),
		newGenerationCmd(get),
		newSetupCmd(get),
		newStatusCmd(get),
		newUninstallCmd(get),
	)

	registerCompletions(cmd, get)
	return cmd
}
