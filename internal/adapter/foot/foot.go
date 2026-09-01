// Package foot declares the generated Foot terminal configuration.
package foot

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dborovcanin/rice/internal/adapter"
)

type Adapter struct{}

// New returns the Foot adapter.
func New() adapter.Adapter { return Adapter{} }

func (Adapter) Name() string { return "foot" }

func (Adapter) Files() []adapter.File {
	return []adapter.File{
		{Template: "foot/foot.ini.tmpl", Path: "foot/foot.ini"},
	}
}

func (Adapter) ConfigPaths() []adapter.ManagedPath {
	return []adapter.ManagedPath{
		{Source: "foot/foot.ini", Target: "foot/foot.ini"},
	}
}

// ReloadMode is new-instances-only: a running foot keeps its palette, and the
// foot server only applies changes to terminals opened afterwards.
func (Adapter) ReloadMode() adapter.ReloadMode { return adapter.ReloadNewInstancesOnly }

// Validate checks the INI shape: every non-comment line outside a section
// header must be a key=value pair, which is the mistake templates actually
// make.
func (Adapter) Validate(dir string) error {
	path := filepath.Join(dir, "foot", "foot.ini")
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("validate foot: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") || strings.HasPrefix(text, ";") {
			continue
		}
		if strings.HasPrefix(text, "[") {
			if !strings.HasSuffix(text, "]") {
				return fmt.Errorf("validate foot: %s:%d: unterminated section header", path, line)
			}
			continue
		}
		if !strings.Contains(text, "=") {
			return fmt.Errorf("validate foot: %s:%d: not a key=value pair: %q", path, line, text)
		}
	}
	return scanner.Err()
}
