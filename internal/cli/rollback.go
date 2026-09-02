package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/dborovcanin/rice/internal/generation"
)

func newRollbackCmd(app func() *App) *cobra.Command {
	return &cobra.Command{
		Use:   "rollback [generation]",
		Short: "Switch back to the previous generation",
		Long: "With no argument, switches to the generation that was current before the\n" +
			"last switch. With a number, switches to that generation.\n\n" +
			"Rollback only moves the `current` symlink; no configuration is rebuilt, so\n" +
			"it returns to the exact bytes that generation was committed with. Rolling\n" +
			"back twice returns to where you started.",
		Example: `  # Undo the last switch.
  rice rollback

  # Go to a specific generation.
  rice generation list
  rice rollback 39`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a := app()
			if err := requireNoPreview(a, "rolling back"); err != nil {
				return err
			}

			target := 0
			if len(args) == 1 {
				n, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("invalid generation %q", args[0])
				}
				target = n
			} else {
				n, err := a.Store.Previous()
				if errors.Is(err, generation.ErrNoGenerations) {
					return errors.New("no previous generation to roll back to")
				}
				if err != nil {
					return err
				}
				target = n
			}

			if err := a.Store.Switch(target); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "current -> generations/%06d\n", target)
			return nil
		},
	}
}
