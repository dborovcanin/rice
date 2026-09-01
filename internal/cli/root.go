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
		root string
		app  *App
	)

	cmd := &cobra.Command{
		Use:     "rice",
		Short:   "Generate consistent configuration for a SwayFX desktop",
		Version: rice.Version,
		Long: "Rice generates complete configuration files for SwayFX, Waybar, Rofi,\n" +
			"Foot, Dunst and swaylock from one theme and one source configuration.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			a, err := NewApp(root)
			if err != nil {
				return err
			}
			app = a
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&root, "root", "",
		"Rice root directory (default $RICE_HOME or ~/.config/rice)")

	get := func() *App { return app }

	cmd.AddCommand(
		newInitCmd(get),
		newApplyCmd(get),
		newRenderCmd(get),
		newRollbackCmd(get),
		newThemeCmd(get),
		newGenerationCmd(get),
	)
	return cmd
}
