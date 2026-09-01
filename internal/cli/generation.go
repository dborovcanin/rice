package cli

import (
	"errors"
	"fmt"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/generation"
)

func newGenerationCmd(app func() *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generation",
		Aliases: []string{"gen"},
		Short:   "Inspect configuration generations",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(
		newGenerationListCmd(app),
		newGenerationCurrentCmd(app),
		newGenerationShowCmd(app),
	)
	return cmd
}

func newGenerationListCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List generations, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			list, err := a.Store.List()
			if err != nil {
				return err
			}
			if len(list) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No generations yet. Run `rice apply`.")
				return nil
			}

			current, _ := a.Store.Current()
			previous, _ := a.Store.Previous()

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for i := len(list) - 1; i >= 0; i-- {
				info := list[i]
				marker := ""
				switch info.Number {
				case current:
					marker = "current"
				case previous:
					marker = "previous"
				}
				created := ""
				if !info.Manifest.CreatedAt.IsZero() {
					created = info.Manifest.CreatedAt.Local().Format(time.RFC3339)
				}
				fmt.Fprintf(w, "%06d\t%s\t%s\t%s\n", info.Number, marker, created, info.Manifest.Summary())
			}
			return w.Flush()
		},
	}
}

func newGenerationCurrentCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the current generation number",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := app().Store.Current()
			if errors.Is(err, generation.ErrNoGenerations) {
				return errors.New("no current generation: run `rice apply`")
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%06d\n", n)
			return nil
		},
	}
}

func newGenerationShowCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "show [number]",
		Short: "Show a generation's manifest",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()

			number := 0
			if len(args) == 1 {
				n, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid generation %q", args[0])
				}
				number = n
			} else {
				n, err := a.Store.Current()
				if err != nil {
					return err
				}
				number = n
			}

			m, err := generation.ReadManifest(a.Paths.Generation(number))
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "generation\t%06d\n", m.Generation)
			fmt.Fprintf(w, "created\t%s\n", m.CreatedAt.Local().Format(time.RFC3339))
			fmt.Fprintf(w, "theme\t%s\n", m.Theme)
			fmt.Fprintf(w, "rice\t%s\n", m.RiceVersion)
			if m.Parent != 0 {
				fmt.Fprintf(w, "parent\t%06d\n", m.Parent)
			}
			if m.Description != "" {
				fmt.Fprintf(w, "description\t%s\n", m.Description)
			}
			fmt.Fprintln(w, "\nfiles\t")
			for _, f := range m.Files {
				fmt.Fprintf(w, "  %s\t%s\treload=%s\n", f.Path, f.SHA256[:12], f.Reload)
			}
			return w.Flush()
		},
	}
}
