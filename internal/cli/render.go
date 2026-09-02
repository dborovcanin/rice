package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newRenderCmd(app func() *App) *cobra.Command {
	var (
		themeName string
		outDir    string
		component string
	)

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render configuration without committing a generation",
		Long: "Renders the configuration to stdout, or to a directory with --output.\n" +
			"Nothing is committed, no generation number is consumed and `current` is\n" +
			"not touched.\n\n" +
			"Use it to see what a theme or a template change would produce before\n" +
			"running apply.",
		Example: `  # Everything, to stdout.
  rice render

  # One component.
  rice render -c waybar

  # Preview a theme without changing config.toml.
  rice render --theme tokyo-night -c sway

  # Write the files somewhere.
  rice render -o /tmp/preview

  # To see what would change, prefer `rice diff`.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()

			cfg, err := a.Config()
			if err != nil {
				return err
			}
			th, err := a.Theme(cfg, themeName)
			if err != nil {
				return err
			}

			if component != "" {
				if _, err := a.Registry.Get(component); err != nil {
					return err
				}
				if !cfg.Components.Enabled(component) {
					return fmt.Errorf("component %q is not enabled in config.toml", component)
				}
			}

			next, err := a.Store.Next()
			if err != nil {
				return err
			}

			files, err := a.Builder.Render(cfg, th, next)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			wrote := 0
			for _, f := range files {
				if component != "" && f.Component != component {
					continue
				}
				wrote++

				if outDir == "" {
					fmt.Fprintf(out, "# ==> %s\n%s\n", f.Path, f.Content)
					continue
				}
				dest := filepath.Join(outDir, filepath.FromSlash(f.Path))
				if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
					return fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
				}
				if err := os.WriteFile(dest, f.Content, f.Mode); err != nil {
					return fmt.Errorf("write %s: %w", dest, err)
				}
				fmt.Fprintln(out, dest)
			}

			if wrote == 0 {
				return fmt.Errorf("component %q generated no files", component)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&themeName, "theme", "", "theme to render with (defaults to config.toml)")
	cmd.Flags().StringVarP(&outDir, "output", "o", "", "write files into this directory instead of stdout")
	cmd.Flags().StringVarP(&component, "component", "c", "", "render only this component")
	return cmd
}
