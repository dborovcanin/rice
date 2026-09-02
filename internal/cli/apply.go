package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/config"
	"github.com/dborovcanin/rice/internal/generation"
	"github.com/dborovcanin/rice/internal/ownership"
	"github.com/dborovcanin/rice/internal/reload"
)

func newApplyCmd(app func() *App) *cobra.Command {
	var (
		themeName   string
		description string
		noSwitch    bool
		noReload    bool
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Build a new generation and make it current",
		Long: "Renders every enabled component, validates the output, commits it as an\n" +
			"immutable generation and points `current` at it.\n\n" +
			"The generation is assembled under a temporary name and renamed into place,\n" +
			"so a template error or a failed validation leaves nothing behind and does\n" +
			"not consume a generation number. Old generations beyond the retention\n" +
			"limit are pruned afterwards, never including the current or previous one.\n\n" +
			"Components already adopted through `rice setup` have their symlinks\n" +
			"repaired and their applications reloaded. Apply never adopts anything on\n" +
			"its own: a path you have not handed to Rice is not touched.",
		Example: `  # Build from config.toml and switch to the result.
  rice apply

  # Record why the generation exists.
  rice apply -m "wider gaps, softer blur"

  # Build with another theme without editing config.toml.
  rice apply --theme tokyo-night

  # Build but keep the current generation active.
  rice apply --no-switch

  # Deploy without poking the running applications.
  rice apply --no-reload`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApply(cmd, app(), applyOptions{
				Theme:       themeName,
				Description: description,
				NoSwitch:    noSwitch,
				NoReload:    noReload,
			})
		},
	}

	cmd.Flags().StringVar(&themeName, "theme", "", "theme to build with (defaults to config.toml)")
	cmd.Flags().StringVarP(&description, "message", "m", "", "description recorded in the manifest")
	cmd.Flags().BoolVar(&noSwitch, "no-switch", false, "build the generation without switching to it")
	cmd.Flags().BoolVar(&noReload, "no-reload", false, "do not reload the applications afterwards")
	return cmd
}

// applyOptions is what an apply needs, shared by `rice apply` and
// `rice theme apply` so the two cannot drift apart.
type applyOptions struct {
	Theme       string
	Description string
	NoSwitch    bool
	NoReload    bool
}

// runApply builds a generation, switches to it, then redeploys and reloads
// whatever is already adopted.
func runApply(cmd *cobra.Command, a *App, opts applyOptions) error {
	if err := requireNoPreview(a, "applying"); err != nil {
		return err
	}

	cfg, err := a.Config()
	if err != nil {
		return err
	}
	th, err := a.Theme(cfg, opts.Theme)
	if err != nil {
		return err
	}

	info, err := a.Store.Create(cfg, th, generation.BuildOptions{
		Description: opts.Description,
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

	if opts.NoSwitch {
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

	return deployAdopted(cmd, a, cfg, opts.NoReload)
}

// deployAdopted repairs the symlinks of already-adopted components and reloads
// them. It deliberately adopts nothing: taking over a path is always an
// explicit `rice setup`, never a side effect of applying.
func deployAdopted(cmd *cobra.Command, a *App, cfg config.Config, noReload bool) error {
	out := cmd.OutOrStdout()

	manifest, err := a.Manifest()
	if err != nil {
		return err
	}
	if len(manifest.Managed) == 0 {
		fmt.Fprintln(out, "\nNothing is adopted yet, so no application configuration was touched.")
		fmt.Fprintln(out, "Run `rice setup` to see what deploying would do.")
		return nil
	}

	plan, err := a.Plan(cfg)
	if err != nil {
		return err
	}
	result, err := ownership.Relink(plan, a.Paths)
	if err != nil {
		return err
	}
	for _, p := range append(result.Linked, result.Relinked...) {
		fmt.Fprintf(out, "relinked %s\n", p)
	}
	for _, s := range result.Skipped {
		fmt.Fprintf(out, "skipped  %s: %s\n", s.Target, s.Reason)
	}

	if noReload {
		return nil
	}

	adapters, err := a.Registry.Select(manifest.Components())
	if err != nil {
		return err
	}
	reports := a.Reload.Reload(cmd.Context(), adapters)

	fmt.Fprintln(out)
	for _, r := range reports {
		if r.Err != nil {
			fmt.Fprintf(out, "%-9s %s: %v\n", r.Component, r.Outcome, r.Err)
			continue
		}
		fmt.Fprintf(out, "%-9s %s\n", r.Component, r.Outcome)
	}
	if failures := reload.Failures(reports); len(failures) > 0 {
		return fmt.Errorf("%d component(s) failed to reload", len(failures))
	}
	return nil
}
