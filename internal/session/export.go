package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/dborovcanin/rice/internal/clipboard"
	"github.com/dborovcanin/rice/internal/generation"
)

// ComponentFiles renders one component's files from the draft.
func (s *Session) ComponentFiles(component string) ([]generation.Rendered, error) {
	if _, err := s.registry.Get(component); err != nil {
		return nil, err
	}
	if !s.Draft.Config.Components.Enabled(component) {
		return nil, fmt.Errorf("%s is not enabled in config.toml", component)
	}

	all, err := s.Render()
	if err != nil {
		return nil, err
	}

	var out []generation.Rendered
	for _, f := range all {
		if f.Component == component {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s generates no files", component)
	}
	return out, nil
}

// ComponentText renders one component as text suitable for pasting elsewhere.
//
// A component with a single file yields that file verbatim, because the point
// of copying is to paste it straight into a configuration. A component with
// several files gets the same `# ==> path` headers `rice render` uses, since
// the alternative is silently concatenating two different formats.
func (s *Session) ComponentText(component string) (string, error) {
	files, err := s.ComponentFiles(component)
	if err != nil {
		return "", err
	}

	if len(files) == 1 {
		return string(files[0].Content), nil
	}

	var b strings.Builder
	for i, f := range files {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "# ==> %s\n%s", f.Path, f.Content)
	}
	return b.String(), nil
}

// ThemeText renders the draft as a theme file, for someone who wants the
// palette rather than a generated configuration.
func (s *Session) ThemeText() (string, error) {
	data, err := toml.Marshal(cloneTheme(s.Draft.Theme))
	if err != nil {
		return "", fmt.Errorf("encode theme: %w", err)
	}
	return string(data), nil
}

// CopyComponent puts one component's generated configuration on the clipboard
// and reports which clipboard tool was used.
func (s *Session) CopyComponent(ctx context.Context, component string) (string, error) {
	text, err := s.ComponentText(component)
	if err != nil {
		return "", err
	}
	return clipboard.Copy(ctx, s.runner, []byte(text))
}

// CopyTheme puts the draft theme file on the clipboard.
func (s *Session) CopyTheme(ctx context.Context) (string, error) {
	text, err := s.ThemeText()
	if err != nil {
		return "", err
	}
	return clipboard.Copy(ctx, s.runner, []byte(text))
}

// CanCopy reports whether any clipboard tool is installed.
func (s *Session) CanCopy() bool { return clipboard.Available(s.runner) }
