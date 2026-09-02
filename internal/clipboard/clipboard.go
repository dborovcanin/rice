// Package clipboard copies generated configuration to the system clipboard,
// so Rice can be used as a configuration generator by someone who does not
// want the rest of the ricing system.
package clipboard

import (
	"context"
	"errors"

	"github.com/dborovcanin/rice/internal/command"
)

// ErrUnavailable is returned when no clipboard tool is installed. The caller
// is expected to fall back to showing the file path or the content itself.
var ErrUnavailable = errors.New("no clipboard tool found: install wl-clipboard, xclip or xsel")

// tool is one way of getting text onto the clipboard.
type tool struct {
	name string
	args []string
}

// tools are tried in order. wl-copy first, because Rice targets Wayland; the
// X11 tools are there for a user running the editor over a forwarded display
// or inside XWayland.
var tools = []tool{
	{name: "wl-copy"},
	{name: "xclip", args: []string{"-selection", "clipboard"}},
	{name: "xsel", args: []string{"--clipboard", "--input"}},
}

// Copy writes content to the clipboard and returns the tool that was used.
func Copy(ctx context.Context, runner command.Runner, content []byte) (string, error) {
	for _, t := range tools {
		if !runner.Look(t.name) {
			continue
		}
		if err := runner.Pipe(ctx, content, t.name, t.args...); err != nil {
			return t.name, err
		}
		return t.name, nil
	}
	return "", ErrUnavailable
}

// Available reports whether any clipboard tool is installed, so an interface
// can hide the action instead of offering it and failing.
func Available(runner command.Runner) bool {
	for _, t := range tools {
		if runner.Look(t.name) {
			return true
		}
	}
	return false
}
