package cli

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/palette"
	"github.com/dborovcanin/rice/internal/session"
	"github.com/dborovcanin/rice/internal/theme"
)

func newThemeFromImageCmd(app func() *App) *cobra.Command {
	var (
		name     string
		variant  string
		clusters int
		contrast float64
		save     bool
		force    bool
		apply    bool
		wall     bool
		from     string
	)

	cmd := &cobra.Command{
		Use:   "from-image <path>",
		Short: "Derive a theme from an image",
		Long: "Reads a wallpaper and writes a theme from the colors in it.\n\n" +
			"The image supplies a background, a readable foreground and three\n" +
			"accents. Everything the theme model can work out for itself — the\n" +
			"surfaces, the muted text, the borders and all sixteen ANSI slots — is\n" +
			"left unset, so the result reads like a hand-written theme and keeps\n" +
			"following its own semantic colors.\n\n" +
			"Foreground contrast is enforced, not hoped for: a wallpaper is no\n" +
			"excuse for an unreadable terminal. Success, warning and error keep\n" +
			"their own hues and borrow only the palette's saturation, because a\n" +
			"green that is not green is worse than one that does not match.\n\n" +
			"An image supplies colours and nothing else, so everything else — the\n" +
			"fonts, the icon and cursor themes, the geometry, the toolkit hints — is\n" +
			"inherited from an existing theme, by default the one config.toml\n" +
			"selects. The result is that theme wearing the image's colours.\n\n" +
			"The result is deterministic: the same image always gives the same\n" +
			"theme. PNG and JPEG are supported.\n\n" +
			"By default the theme is printed. Use --save to write it into the theme\n" +
			"directory, where `rice theme apply` can find it, or --apply to save it,\n" +
			"select it and build a generation in one step.\n\n" +
			"--wallpaper points config.toml at the same image, so the desktop and its\n" +
			"palette come from one file.",
		Example: `  # Look at what an image would give you.
  rice theme from-image ~/Pictures/wallpaper.jpg

  # Keep it.
  rice theme from-image ~/Pictures/wallpaper.jpg --save

  # Name it, and force a light theme out of a dark image.
  rice theme from-image wall.png --name desk --variant light --save

  # The whole job: palette, wallpaper and a generation.
  rice theme from-image ~/Pictures/wall.jpg --apply --wallpaper`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			path := args[0]

			cfg, err := a.Config()
			if err != nil {
				return err
			}

			if name == "" {
				name = themeNameFromImage(path)
			}
			if err := session.ValidThemeName(name); err != nil {
				return err
			}
			switch variant {
			case "", "dark", "light":
			default:
				return fmt.Errorf("variant %q is not \"dark\" or \"light\"", variant)
			}

			// An image cannot say what font to use, so the rest comes from a
			// theme that can.
			base, err := a.Themes.LoadSource(cmp.Or(from, cfg.Theme))
			if err != nil {
				return fmt.Errorf("base theme: %w", err)
			}

			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open image: %w", err)
			}
			defer file.Close()

			th, err := palette.FromReader(file, palette.Options{
				Name:        name,
				Variant:     variant,
				Clusters:    clusters,
				MinContrast: contrast,
				Base:        base,
			})
			if err != nil {
				return err
			}

			// The theme is validated in its resolved form, exactly as a file
			// on disk would be, before it is offered as one.
			if err := th.Resolved().Validate(); err != nil {
				return err
			}

			data, err := theme.Encode(th)
			if err != nil {
				return err
			}

			// Applying and setting the wallpaper both need the theme on disk.
			if apply || wall {
				save = true
			}
			if !save {
				fmt.Fprintf(cmd.OutOrStdout(), "%s", data)
				return nil
			}

			dest := filepath.Join(a.Paths.ThemesDir, name+".toml")
			if _, err := os.Stat(dest); err == nil && !force {
				return fmt.Errorf("%s already exists: pass --force to overwrite it", dest)
			}
			if err := os.MkdirAll(a.Paths.ThemesDir, 0o755); err != nil {
				return fmt.Errorf("create %s: %w", a.Paths.ThemesDir, err)
			}
			if err := os.WriteFile(dest, data, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", dest, err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Wrote %s\n", dest)

			if !apply && !wall {
				fmt.Fprintf(out, "Try it with `rice preview %s`, or keep it with `rice theme apply %s`.\n", name, name)
				return nil
			}

			if wall {
				// An absolute path, because config.toml is read from wherever
				// Rice happens to be run.
				absolute, err := filepath.Abs(path)
				if err != nil {
					return fmt.Errorf("resolve image path: %w", err)
				}
				cfg.Sway.Wallpaper = absolute
				fmt.Fprintf(out, "Wallpaper set to %s\n", absolute)
			}
			if apply {
				cfg.Theme = name
			}
			if err := writeConfig(a, cfg); err != nil {
				return err
			}
			if apply {
				fmt.Fprintf(out, "Theme set to %q\n", name)
			}

			if !apply {
				fmt.Fprintf(out, "Run `rice apply` to build a generation with it.\n")
				return nil
			}
			return runApply(cmd, a, applyOptions{
				Description: "derived from " + filepath.Base(path),
			})
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "theme name (default: the image's file name)")
	cmd.Flags().StringVar(&variant, "variant", "", "force \"dark\" or \"light\" (default: whichever the image suits)")
	cmd.Flags().IntVar(&clusters, "colors", 0, "how many colors to quantize the image to")
	cmd.Flags().Float64Var(&contrast, "min-contrast", 0, "lowest acceptable foreground contrast ratio")
	cmd.Flags().BoolVar(&save, "save", false, "write the theme into the theme directory")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing theme of the same name")
	cmd.Flags().BoolVar(&apply, "apply", false, "save the theme, select it and build a generation")
	cmd.Flags().BoolVar(&wall, "wallpaper", false, "point config.toml at this image as the wallpaper")
	cmd.Flags().StringVar(&from, "from", "", "theme to inherit fonts, icons and geometry from (default: the configured theme)")
	return cmd
}

// themeNameFromImage turns an image path into a usable theme name, so the
// common case needs no --name.
func themeNameFromImage(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))

	// A file name may hold anything; a theme name is a file name Rice has to
	// be able to find again.
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, base)

	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "from-image"
	}
	return clean
}
