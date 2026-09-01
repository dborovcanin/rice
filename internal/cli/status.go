package cli

import (
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/generation"
	"github.com/dborovcanin/rice/internal/ownership"
)

// dependencies are the programs Rice generates configuration or bindings for.
// Missing ones are reported, never installed.
var dependencies = []struct {
	name     string
	binaries []string
}{
	{"compositor", []string{"sway"}},
	{"bar", []string{"waybar"}},
	{"launcher", []string{"rofi"}},
	{"terminal", []string{"foot", "footclient"}},
	{"notifications", []string{"dunst", "dunstctl"}},
	{"lock screen", []string{"swaylock", "swayidle"}},
	{"wallpaper", []string{"swaybg"}},
	{"audio", []string{"wpctl"}},
	{"brightness", []string{"brightnessctl"}},
	{"screenshots", []string{"grim", "slurp"}},
	{"clipboard", []string{"wl-copy", "cliphist"}},
}

func newStatusCmd(app func() *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Aliases: []string{"doctor"},
		Short:   "Report configuration, ownership and dependency state",
		Long: "Prints what Rice sees: the active theme and generation, which\n" +
			"application configuration paths Rice owns, which it would refuse to\n" +
			"touch, and which supporting programs are installed.\n\n" +
			"Status only reads. It never changes anything, so it is the safe way to\n" +
			"find out where things stand.",
		Example: `  rice status
  rice doctor`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

			fmt.Fprintln(out, "Configuration")
			if a.ConfigExists() {
				fmt.Fprintf(w, "  config\t%s\n", a.Paths.ConfigFile)
			} else {
				fmt.Fprintf(w, "  config\t! missing, run `rice init`\n")
			}

			cfg, cfgErr := a.Config()
			if cfgErr != nil {
				fmt.Fprintf(w, "  status\t! %v\n", cfgErr)
				w.Flush()
				return nil
			}
			fmt.Fprintf(w, "  components\t%v\n", cfg.Components.Names())

			th, themeErr := a.Theme(cfg, "")
			if themeErr != nil {
				fmt.Fprintf(w, "  theme\t! %v\n", themeErr)
			} else {
				fmt.Fprintf(w, "  theme\t%s (%s)\n", th.Name, th.Variant)
			}
			w.Flush()

			fmt.Fprintln(out, "\nGenerations")
			list, err := a.Store.List()
			if err != nil {
				return err
			}
			current, currentErr := a.Store.Current()
			switch {
			case errors.Is(currentErr, generation.ErrNoGenerations):
				fmt.Fprintf(w, "  current\t! none, run `rice apply`\n")
			case currentErr != nil:
				fmt.Fprintf(w, "  current\t! %v\n", currentErr)
			default:
				fmt.Fprintf(w, "  current\t%06d\n", current)
			}
			if previous, err := a.Store.Previous(); err == nil {
				fmt.Fprintf(w, "  previous\t%06d\n", previous)
			}
			fmt.Fprintf(w, "  stored\t%d (keeping %d)\n", len(list), cfg.Generations.Keep)
			w.Flush()

			fmt.Fprintf(out, "\nOwnership under %s\n", a.ConfigDir)
			manifest, err := a.Manifest()
			if err != nil {
				return err
			}
			plan, err := a.Plan(cfg)
			if err != nil {
				return err
			}
			for _, action := range plan.Actions {
				marker, note := "=", "rice-managed"
				switch action.Kind {
				case ownership.KindConflict:
					marker, note = "!", action.Reason
				case ownership.KindAdopt:
					marker, note = " ", "your file, not adopted"
				case ownership.KindLink:
					marker, note = " ", "not deployed"
				case ownership.KindRelink:
					marker, note = "!", "link is stale, run `rice apply`"
				}
				fmt.Fprintf(w, "  %s %s\t%s\n", marker, action.Target, note)
			}
			w.Flush()
			if len(manifest.Managed) == 0 {
				fmt.Fprintln(out, "\n  Nothing is adopted. `rice setup` shows what deploying would do.")
			}

			fmt.Fprintln(out, "\nDependencies")
			for _, dep := range dependencies {
				var missing []string
				for _, bin := range dep.binaries {
					if !a.Runner.Look(bin) {
						missing = append(missing, bin)
					}
				}
				if len(missing) == 0 {
					fmt.Fprintf(w, "  ✓ %s\t%v\n", dep.name, dep.binaries)
				} else {
					fmt.Fprintf(w, "  ! %s\tmissing %v\n", dep.name, missing)
				}
			}
			return w.Flush()
		},
	}
	return cmd
}
