package cli

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/session"
	"github.com/dborovcanin/rice/internal/tui"
)

func newTUICmd(app func() *App) *cobra.Command {
	var themeName string

	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Pick and edit a theme interactively",
		Long: "Opens the interactive editor: choose a theme, adjust its colors, fonts,\n" +
			"sizing, icons and cursor, preview any component by running the real\n" +
			"application against the draft, and copy or save the result.\n\n" +
			"The editor writes nothing until you ask it to. Previews render into a\n" +
			"private temporary directory; nothing under ~/.config is touched and\n" +
			"`current` is not moved unless you apply.\n\n" +
			"Running `rice` with no arguments on a terminal opens the same editor.",
		Example: `  rice tui

  # Start from a theme other than the configured one.
  rice tui --theme tokyo-night`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUI(cmd, app(), themeName)
		},
	}

	cmd.Flags().StringVar(&themeName, "theme", "", "theme to start from (defaults to config.toml)")
	return cmd
}

// runTUI builds a session and hands it to the interactive editor.
func runTUI(cmd *cobra.Command, a *App, themeName string) error {
	cfg, err := a.Config()
	if err != nil {
		return err
	}
	if themeName == "" {
		themeName = cfg.Theme
	}

	// The source form is loaded, not the resolved one, so that values a theme
	// leaves derived stay derived while it is edited.
	base, err := a.Themes.LoadSource(themeName)
	if err != nil {
		return err
	}

	s, err := session.New(base, session.Options{
		Themes:    a.Themes,
		Registry:  a.Registry,
		Engine:    a.Engine,
		Runner:    a.Runner,
		Config:    cfg,
		ThemesDir: a.Paths.ThemesDir,
		Version:   a.Builder.Version,
	})
	if err != nil {
		return err
	}
	defer s.Close()

	return tui.Run(tui.Options{
		Session: s,
		Runner:  a.Runner,
		Apply:   applyFromEditor(cmd, a),
	})
}

// applyFromEditor is the editor's route to a real generation. Applying owns
// config.toml, generations, deployment and reload, all of which belong to the
// command tree, so the editor is handed a function rather than those packages.
//
// Output is captured instead of printed: the editor owns the screen while it
// is running, and a half-written command log through the middle of it would be
// unreadable. Failures still surface, as the returned error.
func applyFromEditor(cmd *cobra.Command, a *App) func(string) error {
	return func(themeName string) error {
		th, err := a.Themes.Load(themeName)
		if err != nil {
			return err
		}

		cfg, err := a.Config()
		if err != nil {
			return err
		}
		cfg.Theme = th.Name
		if err := writeConfig(a, cfg); err != nil {
			return err
		}

		quiet := &cobra.Command{}
		quiet.SetContext(cmd.Context())
		var out bytes.Buffer
		quiet.SetOut(&out)
		quiet.SetErr(&out)

		if err := runApply(quiet, a, applyOptions{
			Description: "edited in the interactive editor",
		}); err != nil {
			return fmt.Errorf("%w\n%s", err, out.String())
		}
		return nil
	}
}

// interactive reports whether standard input is a terminal, which is what
// decides whether bare `rice` opens the editor or prints help. A pipe or a
// redirect means the caller wanted output, not an interface.
func interactive() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
