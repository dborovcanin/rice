package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newThemeCmd(app func() *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "theme",
		Short: "Inspect and select themes",
		Long: "A theme carries the whole appearance of the desktop: a semantic palette,\n" +
			"a 16-color ANSI palette, geometry, fonts, icons and cursor.\n\n" +
			"Themes live in ~/.config/rice/themes; a user theme shadows a bundled one\n" +
			"of the same name.",
		Example: `  rice theme list
  rice theme show tokyo-night
  rice theme apply tokyo-night
  rice theme from-image ~/Pictures/wallpaper.jpg`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newThemeListCmd(app),
		newThemeShowCmd(app),
		newThemeCurrentCmd(app),
		newThemeApplyCmd(app),
		newThemeFromImageCmd(app),
	)
	return cmd
}

func newThemeListCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available themes",
		Long: "Lists every theme Rice can load, bundled and user-provided, marking the\n" +
			"one config.toml selects with an asterisk.",
		Example: `  rice theme list`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			entries, err := a.Themes.List()
			if err != nil {
				return err
			}

			active := ""
			if cfg, err := a.Config(); err == nil {
				active = cfg.Theme
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, e := range entries {
				marker := " "
				if e.Name == active {
					marker = "*"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", marker, e.Name, e.Source)
			}
			return w.Flush()
		},
	}
}

func newThemeShowCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show [name]",
		Short: "Show a theme's resolved values",
		Long: "Prints the theme after normalization, so derived colors and defaults\n" +
			"are visible exactly as templates will see them.\n\n" +
			"With no argument it shows the theme config.toml selects. A theme may also\n" +
			"be named by path, which is useful while writing one.",
		Example: `  # The active theme.
  rice theme show

  # Any other bundled or user theme.
  rice theme show catppuccin-mocha

  # A theme file that is not installed yet.
  rice theme show ./my-theme.toml`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			cfg, err := a.Config()
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			th, err := a.Theme(cfg, name)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "name\t%s\n", th.Name)
			fmt.Fprintf(w, "variant\t%s\n", th.Variant)
			if th.Description != "" {
				fmt.Fprintf(w, "description\t%s\n", th.Description)
			}
			fmt.Fprintln(w, "\ncolors\t")
			colors := [][2]string{
				{"background", th.Colors.Background.String()},
				{"surface", th.Colors.Surface.String()},
				{"surface_alt", th.Colors.SurfaceAlt.String()},
				{"overlay", th.Colors.Overlay.String()},
				{"foreground", th.Colors.Foreground.String()},
				{"muted", th.Colors.Muted.String()},
				{"primary", th.Colors.Primary.String()},
				{"secondary", th.Colors.Secondary.String()},
				{"accent", th.Colors.Accent.String()},
				{"success", th.Colors.Success.String()},
				{"warning", th.Colors.Warning.String()},
				{"error", th.Colors.Error.String()},
				{"border", th.Colors.Border.String()},
				{"border_focus", th.Colors.BorderFocus.String()},
			}
			for _, c := range colors {
				fmt.Fprintf(w, "  %s\t%s\n", c[0], c[1])
			}

			fmt.Fprintln(w, "\nui\t")
			fmt.Fprintf(w, "  radius\t%d\n", th.UI.Radius)
			fmt.Fprintf(w, "  border_width\t%d\n", th.UI.BorderWidth)
			fmt.Fprintf(w, "  gaps\t%d inner / %d outer\n", th.UI.GapsInner, th.UI.GapsOuter)
			fmt.Fprintf(w, "  opacity\t%.2f\n", th.UI.Opacity)
			fmt.Fprintf(w, "  blur\t%d radius / %d passes\n", th.UI.BlurRadius, th.UI.BlurPasses)

			fmt.Fprintln(w, "\nfonts\t")
			fmt.Fprintf(w, "  ui\t%s %d\n", th.Fonts.UIFamily, th.Fonts.UISize)
			fmt.Fprintf(w, "  mono\t%s %d\n", th.Fonts.MonoFamily, th.Fonts.MonoSize)
			fmt.Fprintf(w, "  bar\t%s %d\n", th.Fonts.BarFont(), th.Fonts.BarFontSize())

			fmt.Fprintln(w, "\nterminal\t")
			for i := range th.Terminal.Regular {
				fmt.Fprintf(w, "  %d\t%s\t%s\n", i, th.Terminal.Regular[i], th.Terminal.Bright[i])
			}
			return w.Flush()
		},
	}
}

func newThemeCurrentCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the configured theme",
		Long: "Prints the theme name from config.toml, which is the theme the next\n" +
			"apply will use. The theme a committed generation was built from is in its\n" +
			"manifest instead, via `rice generation show`.",
		Example: `  rice theme current`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app().Config()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), cfg.Theme)
			return nil
		},
	}
}

func newThemeApplyCmd(app func() *App) *cobra.Command {
	var (
		description string
		noReload    bool
	)

	cmd := &cobra.Command{
		Use:   "apply <name>",
		Short: "Set the theme in config.toml and build a generation",
		Long: "Writes the theme name into config.toml, then builds a generation and\n" +
			"switches to it. The change is persistent, unlike `rice apply --theme`.\n\n" +
			"The theme is loaded and validated before config.toml is rewritten, so a\n" +
			"bad name leaves the configuration untouched.",
		Example: `  rice theme apply tokyo-night

  # Change your mind.
  rice rollback`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			if err := a.requireConfig(); err != nil {
				return err
			}

			cfg, err := a.Config()
			if err != nil {
				return err
			}
			th, err := a.Themes.Load(args[0])
			if err != nil {
				return err
			}

			cfg.Theme = th.Name
			if err := writeConfig(a, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Theme set to %q\n", th.Name)

			return runApply(cmd, a, applyOptions{
				Description: description,
				NoReload:    noReload,
			})
		},
	}

	cmd.Flags().StringVarP(&description, "message", "m", "", "description recorded in the manifest")
	cmd.Flags().BoolVar(&noReload, "no-reload", false, "do not reload the applications afterwards")
	return cmd
}
