package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/config"
)

func newInitCmd(app func() *App) *cobra.Command {
	var (
		force     bool
		themeName string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the Rice source configuration",
		Long: "Writes a complete config.toml with every default spelled out, so the\n" +
			"file itself documents what Rice can generate, and creates the directories\n" +
			"Rice works in.\n\n" +
			"Nothing outside the Rice root is touched, and an existing config.toml is\n" +
			"never overwritten without --force.",
		Example: `  # Create ~/.config/rice/config.toml with the defaults.
  rice init

  # Start from a particular theme.
  rice init --theme catppuccin-mocha

  # Replace an existing configuration.
  rice init --force`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			if err := a.Paths.EnsureDirs(); err != nil {
				return err
			}

			if _, err := os.Stat(a.Paths.ConfigFile); err == nil && !force {
				return fmt.Errorf("%s already exists: pass --force to overwrite", a.Paths.ConfigFile)
			} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("stat config: %w", err)
			}

			cfg := config.DefaultConfig()
			if themeName != "" {
				cfg.Theme = themeName
			}
			if _, err := a.Themes.Load(cfg.Theme); err != nil {
				return err
			}

			data, err := config.Marshal(cfg)
			if err != nil {
				return err
			}
			header := "# Rice source configuration. Appearance lives in themes; structure lives here.\n" +
				"# Run `rice apply` after editing.\n\n"
			if err := os.WriteFile(a.Paths.ConfigFile, append([]byte(header), data...), 0o644); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", a.Paths.ConfigFile)
			fmt.Fprintf(cmd.OutOrStdout(), "Theme: %s\n", cfg.Theme)
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext: rice apply\n")
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config.toml")
	cmd.Flags().StringVar(&themeName, "theme", "", "theme to start from")
	return cmd
}
