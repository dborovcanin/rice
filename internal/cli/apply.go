package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/generation"
)

func newApplyCmd(app func() *App) *cobra.Command {
	var (
		themeName   string
		description string
		noSwitch    bool
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Build a new generation and make it current",
		Long: "Renders every enabled component, validates the output, commits it as an\n" +
			"immutable generation and points `current` at it.\n\n" +
			"The generation is assembled under a temporary name and renamed into place,\n" +
			"so a template error or a failed validation leaves nothing behind and does\n" +
			"not consume a generation number. Old generations beyond the retention\n" +
			"limit are pruned afterwards, never including the current or previous one.",
		Example: `  # Build from config.toml and switch to the result.
  rice apply

  # Record why the generation exists.
  rice apply -m "wider gaps, softer blur"

  # Build with another theme without editing config.toml.
  rice apply --theme tokyo-night

  # Build but keep the current generation active.
  rice apply --no-switch`,
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

			info, err := a.Store.Create(cfg, th, generation.BuildOptions{
				Description: description,
				ThemeSource: th.Name,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Generation %06d built from theme %q\n", info.Number, th.Name)
			for _, f := range info.Manifest.Files {
				fmt.Fprintf(out, "  %s\n", f.Path)
			}

			if noSwitch {
				fmt.Fprintf(out, "\ncurrent left unchanged (--no-switch)\n")
				return nil
			}
			if err := a.Store.Switch(info.Number); err != nil {
				return err
			}
			fmt.Fprintf(out, "\ncurrent -> generations/%06d\n", info.Number)

			removed, err := a.Store.Prune(cfg.Generations.Keep)
			if err != nil {
				return err
			}
			if len(removed) > 0 {
				fmt.Fprintf(out, "pruned %d old generation(s)\n", len(removed))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&themeName, "theme", "", "theme to build with (defaults to config.toml)")
	cmd.Flags().StringVarP(&description, "message", "m", "", "description recorded in the manifest")
	cmd.Flags().BoolVar(&noSwitch, "no-switch", false, "build the generation without switching to it")
	return cmd
}
