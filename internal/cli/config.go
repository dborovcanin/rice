package cli

import (
	"fmt"
	"os"

	"github.com/dborovcanin/rice/internal/config"
)

// writeConfig rewrites config.toml from a config value. It is used by commands
// that change one setting, such as `rice theme apply`.
func writeConfig(a *App, cfg config.Config) error {
	data, err := config.Marshal(cfg)
	if err != nil {
		return err
	}
	header := "# Rice source configuration. Appearance lives in themes; structure lives here.\n" +
		"# Run `rice apply` after editing.\n\n"
	if err := os.WriteFile(a.Paths.ConfigFile, append([]byte(header), data...), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
